package retrieval

import (
	"context"
	"fmt"
	"github.com/application-research/filclient"
	"github.com/application-research/filclient/rep"
	"github.com/application-research/filclient/retrievehelper"
	"github.com/filecoin-project/go-address"
	datatransfer "github.com/filecoin-project/go-data-transfer"
	"github.com/filecoin-project/go-fil-markets/retrievalmarket"
	"github.com/ipfs/go-cid"
	flatfs "github.com/ipfs/go-ds-flatfs"
	levelds "github.com/ipfs/go-ds-leveldb"
	blockstore "github.com/ipfs/go-ipfs-blockstore"
	basicnode "github.com/ipld/go-ipld-prime/node/basic"
	"github.com/ipld/go-ipld-prime/traversal/selector"
	"github.com/ipld/go-ipld-prime/traversal/selector/builder"
	"github.com/libp2p/go-libp2p"
	"github.com/mitchellh/go-homedir"
	"os"
	"sort"
	"time"
	"validation-bot/worker/model"
)

type timeBytesPair struct {
	time  time.Time
	bytes uint64
}

type timeEventPair struct {
	time  time.Time
	event rep.RetrievalEvent
}

type metrics struct {
	retrievalEvents []timeEventPair
	bytesReceived   []timeBytesPair
}

type retrievalHelper struct {
	maxAllowedDownloadBytes uint64
	metrics                 *metrics
	done                    chan interface{}
}

func (r retrievalHelper) OnRetrievalEvent(event rep.RetrievalEvent) {
	r.metrics.retrievalEvents = append(r.metrics.retrievalEvents, timeEventPair{time.Now(), event})
	fmt.Printf("[%s]RetrievalEvent - code: %s, phase: %s\n", time.Now().UTC().Format(time.RFC3339Nano), event.Code(), event.Phase())
	if event.Code() == rep.FailureCode || event.Code() == rep.SuccessCode {
		r.done <- event.Code()
	}
}

func getRetrievalEventByCode(eventList []timeEventPair, code rep.Code) *timeEventPair {
	for _, event := range eventList {
		if event.event.Code() == code {
			return &event
		}
	}
	return nil
}

func (r retrievalHelper) CalculateValidationResult() *model.ValidationResult {
	if event := getRetrievalEventByCode(r.metrics.retrievalEvents, rep.FailureCode); event != nil {
		return &model.ValidationResult{
			Success:      false,
			ErrorCode:    model.RetrievalFailed,
			ErrorMessage: event.event.(rep.RetrievalEventFailure).ErrorMessage(),
		}
	}
	// Calculate Time to first byte and average speed
	ttfb := int64(0)
	avgSpeed := 0.0
	accepted := getRetrievalEventByCode(r.metrics.retrievalEvents, rep.AcceptedCode)
	firstByte := getRetrievalEventByCode(r.metrics.retrievalEvents, rep.FirstByteCode)
	if accepted != nil && firstByte != nil {
		ttfb = firstByte.time.Sub(accepted.time).Milliseconds()
		if len(r.metrics.bytesReceived) > 0 {
			avgSpeed = float64(r.metrics.bytesReceived[len(r.metrics.bytesReceived)-1].bytes) /
				(r.metrics.bytesReceived[len(r.metrics.bytesReceived)-1].time.Sub(firstByte.time).Seconds())
		}
	}
	// Calculate bytes downloaded
	bytesDownloaded := uint64(0)
	if len(r.metrics.bytesReceived) > 0 {
		bytesDownloaded = r.metrics.bytesReceived[len(r.metrics.bytesReceived)-1].bytes
	}

	// Calculate speed percentiles
	speeds := make([]float64, 0)
	if len(r.metrics.bytesReceived) > 0 {
		lastTime := r.metrics.bytesReceived[0].time
		lastBytes := r.metrics.bytesReceived[0].bytes
		for i := 1; i < len(r.metrics.bytesReceived); i++ {
			currentTime := r.metrics.bytesReceived[i].time
			currentBytes := r.metrics.bytesReceived[i].bytes
			if i == len(r.metrics.bytesReceived)-1 || currentTime.Sub(lastTime).Seconds() >= 1 {
				speeds = append(speeds, float64(currentBytes-lastBytes)/currentTime.Sub(lastTime).Seconds())
				lastTime = currentTime
				lastBytes = currentBytes
			}
		}
	}
	sort.Slice(speeds, func(i, j int) bool {
		return speeds[i] < speeds[j]
	})
	// Calculate percentiles 1, 5, 10, 25, 50, 75, 90, 95, 99
	pTargets := [9]float64{0.01, 0.05, 0.1, 0.25, 0.5, 0.75, 0.9, 0.95, 0.99}
	percentiles := [9]float64{}
	if len(speeds) > 0 {
		for i, pTarget := range pTargets {
			index := int(float64(len(speeds)) * pTarget)
			percentiles[i] = speeds[index]
		}
	}
	return &model.ValidationResult{
		Success:           true,
		BytesDownloaded:   bytesDownloaded,
		TimeToFirstByteMs: ttfb,
		SpeedBpsAvg:       avgSpeed,
		SpeedBpsP1:        percentiles[0],
		SpeedBpsP5:        percentiles[1],
		SpeedBpsP10:       percentiles[2],
		SpeedBpsP25:       percentiles[3],
		SpeedBpsP50:       percentiles[4],
		SpeedBpsP75:       percentiles[5],
		SpeedBpsP90:       percentiles[6],
		SpeedBpsP95:       percentiles[7],
		SpeedBpsP99:       percentiles[8],
	}
}

func (r retrievalHelper) onBytesReceived(bytesReceived uint64) {
	// Register the bytes received at the current time
	r.metrics.bytesReceived = append(r.metrics.bytesReceived, timeBytesPair{
		time.Now(), bytesReceived,
	})
	// fmt.Printf("[%s]DataTransfered - %d\n", time.Now().UTC().Format(time.RFC3339Nano), bytesReceived)
	// Consider hitting max download bytes as a success event and should early terminate
	if bytesReceived >= r.maxAllowedDownloadBytes {
		r.done <- rep.SuccessCode
	}
}

type GraphSyncRetrievalHandler struct {
	model.RetrievalHandler
}

func (g GraphSyncRetrievalHandler) HandleRetrieval(ctx context.Context, message model.ValidationInput) (*model.ValidationResult, error) {
	c, err := cid.Decode(message.Cid)
	if err != nil {
		return nil, fmt.Errorf("failed to decode cid %s: %w", message.Cid, err)
	}

	provider, err := address.NewFromString(message.Provider)
	if err != nil {
		return nil, fmt.Errorf("failed to parse provider address %s: %w", message.Provider, err)
	}

	ddir, err := homedir.Expand("~/.filc")
	if err != nil {
		return nil, fmt.Errorf("failed to expand home dir: %w", err)
	}

	if err := os.MkdirAll(ddir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create data dir %s: %w", ddir, err)
	}

	fc, closer, err := getFilClient(ctx, ddir)
	if err != nil {
		return nil, fmt.Errorf("failed to create client: %w", err)
	}

	defer closer()
	helper := retrievalHelper{metrics: &metrics{
		retrievalEvents: make([]timeEventPair, 0),
		bytesReceived:   make([]timeBytesPair, 0),
	}, done: make(chan interface{}), maxAllowedDownloadBytes: message.MaxDownloadBytes}
	fc.SubscribeToRetrievalEvents(helper)
	eventsToLog := []datatransfer.EventCode{
		datatransfer.Cancel,
		datatransfer.Error,
		datatransfer.PauseInitiator,
		datatransfer.PauseResponder,
		datatransfer.Disconnected,
		datatransfer.RequestTimedOut,
		datatransfer.SendDataError,
		datatransfer.ReceiveDataError,
		datatransfer.RequestCancelled,
	}
	isEventToLog := func(i datatransfer.EventCode) bool {
		for _, e := range eventsToLog {
			if e == i {
				return true
			}
		}
		return false
	}
	fc.SubscribeToDataTransferEvents(func(event datatransfer.Event, channelState datatransfer.ChannelState) {
		if isEventToLog(event.Code) {
			fmt.Printf(
				"[%s]DataTransferEvent - EventCode: %s, EventMessage: %s, ChannelStatus: %s, ChannelMessage: %s\n",
				event.Timestamp.UTC().Format(time.RFC3339Nano),
				datatransfer.Events[event.Code],
				event.Message,
				datatransfer.Statuses[channelState.Status()],
				channelState.Message())
		}
	})

	// Selector Node
	ssb := builder.NewSelectorSpecBuilder(basicnode.Prototype.Any)
	selectorNode := ssb.ExploreRecursive(
		selector.RecursionLimitDepth(3),
		ssb.ExploreAll(ssb.ExploreRecursiveEdge())).Node()
	// -> above does not work, "data transfer failed: datatransfer error: response rejected"
	selectorNode = nil

	go func() {
		query, err := fc.RetrievalQuery(ctx, provider, c)
		if err != nil {
			helper.done <- &model.ValidationResult{
				Success:      false,
				ErrorCode:    model.RetrievalQueryFailed,
				ErrorMessage: err.Error(),
			}
			return
		}
		if query.Status == retrievalmarket.QueryResponseUnavailable {
			helper.done <- &model.ValidationResult{
				Success:      false,
				ErrorCode:    model.QueryResponseUnavailable,
				ErrorMessage: query.Message,
			}
			return
		} else if query.Status == retrievalmarket.QueryResponseError {
			helper.done <- &model.ValidationResult{
				Success:      false,
				ErrorCode:    model.QueryResponseError,
				ErrorMessage: query.Message,
			}
			return
		}

		proposal, err := retrievehelper.RetrievalProposalForAsk(query, c, selectorNode)
		if err != nil {
			helper.done <- &model.ValidationResult{
				Success:      false,
				ErrorCode:    model.InternalError,
				ErrorMessage: err.Error(),
			}
			return
		}
		_, err = fc.RetrieveContentWithProgressCallback(
			ctx,
			provider,
			proposal,
			func(bytesReceived uint64) {
				helper.onBytesReceived(bytesReceived)
			},
		)
		if err != nil {
			helper.done <- &model.ValidationResult{
				Success:      false,
				ErrorCode:    model.RetrievalFailed,
				ErrorMessage: err.Error(),
			}
			return
		}
	}()
	// If max duration seconds is reached, early terminate with the current stats
	select {
	case f := <-helper.done:
		fc.Libp2pTransferMgr.Stop()
		switch f.(type) {
		case rep.Code:
			return helper.CalculateValidationResult(), nil
		case *model.ValidationResult:
			return f.(*model.ValidationResult), nil
		default:
			return nil, fmt.Errorf("unexpected type %T", f)
		}
	case <-time.After(time.Duration(message.MaxDurationSeconds) * time.Second):
		fc.Libp2pTransferMgr.Stop()
		return helper.CalculateValidationResult(), nil
	}
}

func getFilClient(ctx context.Context, cfgdir string) (*filclient.FilClient, func(), error) {
	peerkey, err := LoadOrInitPeerKey(keyPath(cfgdir))
	if err != nil {
		return nil, nil, err
	}

	h, err := libp2p.New(
		libp2p.ListenAddrStrings("/ip4/0.0.0.0/tcp/6755"),
		libp2p.Identity(peerkey),
	)
	if err != nil {
		return nil, nil, err
	}

	bstoreDatastore, err := flatfs.CreateOrOpen(blockstorePath(cfgdir), flatfs.NextToLast(3), false)
	bstore := blockstore.NewBlockstoreNoPrefix(bstoreDatastore)
	if err != nil {
		return nil, nil, fmt.Errorf("blockstore could not be opened (it may be incompatible after an update - try delete the blockstore and try again): %v", err)
	}

	ds, err := levelds.NewDatastore(datastorePath(cfgdir), nil)
	if err != nil {
		return nil, nil, err
	}

	wallet, err := setupWallet(walletPath(cfgdir))
	if err != nil {
		return nil, nil, err
	}

	api, closer, err := GetGatewayAPI(ctx)
	if err != nil {
		return nil, nil, err
	}

	addr, err := wallet.GetDefault()
	if err != nil {
		return nil, nil, err
	}

	fc, err := filclient.NewClient(h, api, wallet, addr, bstore, ds, cfgdir)
	if err != nil {
		return nil, nil, err
	}

	return fc, closer, nil
}
