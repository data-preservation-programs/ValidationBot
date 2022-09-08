package retrieval

import (
	"context"
	"fmt"
	"github.com/application-research/filclient"
	"github.com/application-research/filclient/rep"
	"github.com/application-research/filclient/retrievehelper"
	"github.com/filecoin-project/go-address"
	"github.com/filecoin-project/go-fil-markets/retrievalmarket"
	"github.com/ipfs/go-cid"
	flatfs "github.com/ipfs/go-ds-flatfs"
	levelds "github.com/ipfs/go-ds-leveldb"
	blockstore "github.com/ipfs/go-ipfs-blockstore"
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

type metrics struct {
	retrievalEvents map[rep.Code]time.Time
	bytesReceived   []timeBytesPair
}

type retrievalHelper struct {
	maxAllowedDownloadBytes uint64
	metrics                 metrics
	done                    chan interface{}
	finished                bool // do we need a mutex
}

func (r retrievalHelper) OnRetrievalEvent(event rep.RetrievalEvent) {
	if r.finished {
		return
	}
	r.metrics.retrievalEvents[event.Code()] = time.Now()
	fmt.Printf("[%s]RetrievalEvent - code: %s, phase: %s\n", time.Now().UTC().Format(time.RFC3339Nano), event.Code(), event.Phase())
	if event.Code() == rep.FailureCode || event.Code() == rep.SuccessCode {
		r.done <- event.Code()
		r.finished = true
	}
}

func (r *retrievalHelper) CalculateValidationResult() *model.ValidationResult {
	if _, ok := r.metrics.retrievalEvents[rep.FailureCode]; ok {
		return &model.ValidationResult{
			Success:   false,
			ErrorCode: model.RetrievalFailed,
		}
	}
	// Calculate Time to first byte and average speed
	ttfb := int64(0)
	avgSpeed := 0.0
	if acceptedTime, ok := r.metrics.retrievalEvents[rep.AcceptedCode]; ok {
		if firstByteTime, ok := r.metrics.retrievalEvents[rep.FirstByteCode]; ok {
			ttfb = firstByteTime.Sub(acceptedTime).Milliseconds()
			if len(r.metrics.bytesReceived) > 0 {
				avgSpeed = float64(r.metrics.bytesReceived[len(r.metrics.bytesReceived)-1].bytes) /
					(r.metrics.bytesReceived[len(r.metrics.bytesReceived)-1].time.Sub(firstByteTime).Seconds())
			}
		}
	}
	speeds := make([]float64, 0)
	for i := 1; i < len(r.metrics.bytesReceived); i++ {
		speeds = append(speeds,
			(float64(r.metrics.bytesReceived[i].bytes-r.metrics.bytesReceived[i-1].bytes))/
				(r.metrics.bytesReceived[i].time.Sub(r.metrics.bytesReceived[i-1].time).Seconds()),
		)
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
		TimeToFirstByteMs: ttfb,
		AverageSpeedBps:   avgSpeed,
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

func (r *retrievalHelper) onBytesReceived(bytesReceived uint64) {
	if r.finished {
		return
	}
	// Register the bytes received at the current time
	r.metrics.bytesReceived = append(r.metrics.bytesReceived, timeBytesPair{
		time.Now(), bytesReceived,
	})
	// fmt.Printf("[%s]DataTransfered - %d\n", time.Now().UTC().Format(time.RFC3339Nano), bytesReceived)
	// Consider hitting max download bytes as a success event and should early terminate
	if bytesReceived >= r.maxAllowedDownloadBytes {
		r.done <- rep.SuccessCode
		r.finished = true
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
	helper := retrievalHelper{metrics: metrics{
		retrievalEvents: make(map[rep.Code]time.Time),
		bytesReceived:   make([]timeBytesPair, 0),
	}, done: make(chan interface{}), maxAllowedDownloadBytes: message.MaxDownloadBytes}
	fc.SubscribeToRetrievalEvents(helper)
	go func() {
		query, err := fc.RetrievalQuery(ctx, provider, c)
		if err != nil {
			helper.done <- &model.ValidationResult{
				Success:      false,
				ErrorCode:    model.RetrievalQueryFailed,
				ErrorMessage: err.Error(),
			}
			helper.finished = true
			return
		}
		if query.Status == retrievalmarket.QueryResponseUnavailable {
			helper.done <- &model.ValidationResult{
				Success:      false,
				ErrorCode:    model.QueryResponseUnavailable,
				ErrorMessage: query.Message,
			}
			helper.finished = true
			return
		} else if query.Status == retrievalmarket.QueryResponseError {
			helper.done <- &model.ValidationResult{
				Success:      false,
				ErrorCode:    model.QueryResponseError,
				ErrorMessage: query.Message,
			}
			helper.finished = true
			return
		}

		proposal, err := retrievehelper.RetrievalProposalForAsk(query, c, nil)
		if err != nil {
			helper.done <- &model.ValidationResult{
				Success:      false,
				ErrorCode:    model.InternalError,
				ErrorMessage: err.Error(),
			}
			helper.finished = true
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
			helper.finished = true
			return
		}
	}()
	// If max duration seconds is reached, early terminate with the current stats
	select {
	case f := <-helper.done:
		switch f.(type) {
		case rep.Code:
			return helper.CalculateValidationResult(), nil
		case *model.ValidationResult:
			return f.(*model.ValidationResult), nil
		}
	case <-time.After(time.Duration(message.MaxDurationSeconds) * time.Second):
		return helper.CalculateValidationResult(), nil
	}
	return nil, nil
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
