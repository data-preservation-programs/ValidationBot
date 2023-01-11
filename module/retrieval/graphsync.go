package retrieval

import (
	"context"
	"os"
	"path/filepath"
	"time"

	"validation-bot/role"

	"github.com/application-research/filclient"
	"github.com/application-research/filclient/keystore"
	"github.com/application-research/filclient/rep"
	"github.com/application-research/filclient/retrievehelper"
	"github.com/filecoin-project/go-address"
	datatransfer "github.com/filecoin-project/go-data-transfer"
	"github.com/filecoin-project/go-fil-markets/retrievalmarket"
	"github.com/filecoin-project/lotus/api"
	"github.com/filecoin-project/lotus/chain/types"
	"github.com/filecoin-project/lotus/chain/wallet"
	"github.com/google/uuid"
	"github.com/ipfs/go-cid"
	flatfs "github.com/ipfs/go-ds-flatfs"
	levelds "github.com/ipfs/go-ds-leveldb"
	blockstore "github.com/ipfs/go-ipfs-blockstore"
	"github.com/libp2p/go-libp2p/core/host"
	"github.com/pkg/errors"
	"github.com/rs/zerolog"
	log2 "github.com/rs/zerolog/log"
	"golang.org/x/exp/slices"
)

type Cleanup func()

type GraphSyncRetriever interface {
	Retrieve(
		parent context.Context,
		minerAddress address.Address,
		dataCid cid.Cid,
		timeout time.Duration,
	) (*ResultContent, error)
}

type GraphSyncRetrieverBuilder interface {
	Build() (GraphSyncRetriever, Cleanup, error)
}

type GraphSyncRetrieverImpl struct {
	log      zerolog.Logger
	lotusAPI api.Gateway
	libp2p   host.Host
	tmpDir   string
}

type GraphSyncRetrieverBuilderImpl struct {
	LotusAPI api.Gateway
	BaseDir  string
}

func (g GraphSyncRetrieverBuilderImpl) Build() (GraphSyncRetriever, Cleanup, error) {
	libp2p, err := role.NewLibp2pHostWithRandomIdentityAndPort()
	if err != nil {
		return nil, nil, errors.Wrap(err, "failed to create libp2p host")
	}

	tmpdir := filepath.Join(g.BaseDir, uuid.New().String())

	err = os.MkdirAll(tmpdir, 0o755)
	if err != nil {
		return nil, nil, errors.Wrap(err, "failed to create temp folder")
	}

	retriever := GraphSyncRetrieverImpl{
		log:      log2.With().Str("module", "retrieval_graphsync").Caller().Logger(),
		lotusAPI: g.LotusAPI,
		libp2p:   libp2p,
		tmpDir:   tmpdir,
	}

	return retriever, func() {
		libp2p.Close()
		os.RemoveAll(tmpdir)
	}, nil
}

// TODO: Generalize this across retreival?
type retrievalStats struct {
	log           zerolog.Logger
	events        []TimeEventPair
	done          chan interface{}
	firstByteCode string
	protocol      Protocol
}

func (r *retrievalStats) NewResultContent(status ResultStatus, errorMessage string) *ResultContent {
	r.log.Debug().Str("status", string(status)).Str("errorMessage", errorMessage).Msg("calculate stats for new result")
	var bytesDownloaded uint64
	var startTime time.Time
	var firstByteTime time.Time
	var lastEventTime time.Time

	for _, event := range r.events {
		if event.Timestamp.Before(startTime) || startTime.IsZero() {
			startTime = event.Timestamp
		}

		if event.Timestamp.After(lastEventTime) {
			lastEventTime = event.Timestamp
		}

		if event.Received > bytesDownloaded {
			bytesDownloaded = event.Received
		}

		if event.Code == r.firstByteCode {
			firstByteTime = event.Timestamp
		}
	}

	var averageSpeedPerSec float64
	var timeToFirstByte time.Duration

	timeElapsedForDownload := lastEventTime.Sub(firstByteTime)
	if timeElapsedForDownload > 0 && bytesDownloaded > 0 {
		averageSpeedPerSec = float64(bytesDownloaded) / timeElapsedForDownload.Seconds()
	}

	if !firstByteTime.IsZero() {
		timeToFirstByte = firstByteTime.Sub(startTime)
	}

	return &ResultContent{
		Status:       status,
		ErrorMessage: errorMessage,
		CalculatedStats: CalculatedStats{
			Events:             r.events,
			BytesDownloaded:    bytesDownloaded,
			AverageSpeedPerSec: averageSpeedPerSec,
			TimeElapsed:        lastEventTime.Sub(startTime),
			TimeToFirstByte:    timeToFirstByte,
		},
		Protocol: r.protocol,
	}
}

func (r *retrievalStats) OnRetrievalEvent(event rep.RetrievalEvent) {
	r.log.Debug().Str("event_type", "retrieval_event").Str("code", string(event.Code())).Msg(string(event.Phase()))
	r.events = append(
		r.events, TimeEventPair{
			Timestamp: time.Now(),
			Code:      string(event.Code()),
			Message:   string(event.Phase()),
			Received:  0,
		},
	)

	if event.Code() == rep.FailureCode || event.Code() == rep.SuccessCode {
		r.done <- event
	}
}

func (r *retrievalStats) OnDataTransferEvent(event datatransfer.Event, channelState datatransfer.ChannelState) {
	if event.Code == datatransfer.DataReceived && channelState.Received() == 0 {
		return
	}

	r.log.Debug().
		Str("event_type", "data_transfer").
		Str("code", datatransfer.Events[event.Code]).
		Uint64("Received", channelState.Received()).
		Msg(event.Message)

	errorEvents := []datatransfer.EventCode{
		datatransfer.Cancel,
		datatransfer.Error,
		datatransfer.Disconnected,
		datatransfer.RequestTimedOut,
		datatransfer.SendDataError,
		datatransfer.ReceiveDataError,
		datatransfer.RequestCancelled,
	}

	r.events = append(
		r.events, TimeEventPair{
			Timestamp: event.Timestamp,
			Code:      datatransfer.Events[event.Code],
			Message:   event.Message,
			Received:  channelState.Received(),
		},
	)
	if slices.Contains(errorEvents, event.Code) {
		r.done <- event
	}
}

func setupWallet(ctx context.Context, dir string) (*wallet.LocalWallet, error) {
	kstore, err := keystore.OpenOrInitKeystore(dir)
	if err != nil {
		return nil, errors.Wrap(err, "failed to open keystore")
	}

	wallet, err := wallet.NewWallet(kstore)
	if err != nil {
		return nil, errors.Wrap(err, "failed to create wallet")
	}

	addrs, err := wallet.WalletList(ctx)
	if err != nil {
		return nil, errors.Wrap(err, "failed to list wallet addresses")
	}

	if len(addrs) == 0 {
		_, err := wallet.WalletNew(ctx, types.KTBLS)
		if err != nil {
			return nil, errors.Wrap(err, "failed to create new wallet address")
		}
	}

	return wallet, nil
}

func (g GraphSyncRetrieverImpl) newFilClient(ctx context.Context, baseDir string) (
	*filclient.FilClient,
	Cleanup,
	error,
) {
	//nolint:gomnd
	bstoreDatastore, err := flatfs.CreateOrOpen(filepath.Join(baseDir, "blockstore"), flatfs.NextToLast(3), false)
	if err != nil {
		return nil, nil, errors.Wrap(err, "cannot create or open flatfs blockstore")
	}

	bstore := blockstore.NewBlockstoreNoPrefix(bstoreDatastore)

	datastore, err := levelds.NewDatastore(filepath.Join(baseDir, "datastore"), nil)
	if err != nil {
		bstoreDatastore.Close()
		return nil, nil, errors.Wrap(err, "cannot create leveldb datastore")
	}

	closer := func() {
		bstoreDatastore.Close()
		datastore.Close()
	}

	wallet, err := setupWallet(ctx, filepath.Join(baseDir, "wallet"))
	if err != nil {
		closer()
		return nil, nil, errors.Wrap(err, "cannot create wallet")
	}

	addr, err := wallet.GetDefault()
	if err != nil {
		closer()
		return nil, nil, errors.Wrap(err, "cannot get default wallet address")
	}

	fClient, err := filclient.NewClient(g.libp2p, g.lotusAPI, wallet, addr, bstore, datastore, baseDir)
	if err != nil {
		closer()
		return nil, nil, errors.Wrap(err, "cannot create filclient")
	}

	return fClient, closer, nil
}

func (g GraphSyncRetrieverImpl) Retrieve(
	parent context.Context,
	minerAddress address.Address,
	dataCid cid.Cid,
	timeout time.Duration,
) (*ResultContent, error) {
	g.log.Debug().Str("provider", minerAddress.String()).Msg("retrieving miner info")

	ctx, cancel := context.WithTimeout(parent, timeout)
	defer cancel()

	filClient, cleanup, err := g.newFilClient(ctx, g.tmpDir)
	if err != nil {
		return nil, errors.Wrap(err, "cannot create filClient")
	}

	if cleanup != nil {
		defer cleanup()
	}

	stats := &retrievalStats{
		log:           g.log,
		done:          make(chan interface{}),
		events:        make([]TimeEventPair, 0),
		firstByteCode: string(rep.FirstByteCode),
		protocol:      GraphSync,
	}
	filClient.SubscribeToRetrievalEvents(stats)
	filClient.SubscribeToDataTransferEvents(stats.OnDataTransferEvent)

	go func() {
		g.log.Debug().Str("provider", minerAddress.String()).Str(
			"dataCid",
			dataCid.String(),
		).Msg("sending retrieval query")

		query, err := filClient.RetrievalQuery(ctx, minerAddress, dataCid)
		if err != nil {
			stats.done <- ResultContent{
				Status:       QueryFailure,
				ErrorMessage: err.Error(),
			}
			return
		}

		if query.Status == retrievalmarket.QueryResponseUnavailable {
			stats.done <- ResultContent{
				Status:       QueryResponseUnavailable,
				ErrorMessage: query.Message,
			}
			return
		} else if query.Status == retrievalmarket.QueryResponseError {
			stats.done <- ResultContent{
				Status:       QueryResponseError,
				ErrorMessage: query.Message,
			}
			return
		}

		proposal, err := retrievehelper.RetrievalProposalForAsk(query, dataCid, nil)
		if err != nil {
			stats.done <- ResultContent{
				Status:       ProposalFailure,
				ErrorMessage: err.Error(),
			}
			return
		}

		g.log.Debug().Str("provider", minerAddress.String()).Str(
			"dataCid",
			dataCid.String(),
		).Msg("start retrieving content")

		_, err = filClient.RetrieveContent(ctx, minerAddress, proposal)
		if err != nil {
			stats.done <- ResultContent{
				Status:       RetrieveFailure,
				ErrorMessage: err.Error(),
			}
			return
		}
	}()
	select {
	case <-ctx.Done():
		return stats.NewResultContent(RetrieveTimeout, ""), nil
	case result := <-stats.done:
		switch result := result.(type) {
		case ResultContent:
			return stats.NewResultContent(result.Status, result.ErrorMessage), nil
		case rep.RetrievalEvent:
			if result.Code() == rep.SuccessCode {
				return stats.NewResultContent(Success, string(result.Phase())), nil
			}

			return stats.NewResultContent(RetrieveFailure, string(result.Phase())), nil
		case datatransfer.Event:
			return stats.NewResultContent(DataTransferFailure, result.Message), nil
		default:
			return nil, errors.New("unknown result type")
		}
	}
}
