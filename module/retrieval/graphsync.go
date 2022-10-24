package retrieval

import (
	"context"
	"os"
	"path/filepath"
	"time"
	"validation-bot/module"
	"validation-bot/role"

	"github.com/application-research/filclient"
	"github.com/application-research/filclient/keystore"
	"github.com/application-research/filclient/rep"
	"github.com/application-research/filclient/retrievehelper"
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
	log "github.com/rs/zerolog/log"
	"github.com/stretchr/testify/mock"
	"golang.org/x/exp/slices"
)

type Cleanup = func()

type GraphSyncRetriever interface {
	Retrieve(parent context.Context, provider string, dataCid cid.Cid, timeout time.Duration) (*ResultContent, error)
}

type MockGraphSyncRetriever struct {
	mock.Mock
}

func (m *MockGraphSyncRetriever) Retrieve(parent context.Context, provider string, dataCid cid.Cid, timeout time.Duration) (*ResultContent, error) {
	args := m.Called(parent, provider, dataCid, timeout)
	return args.Get(0).(*ResultContent), args.Error(1)
}

type GraphSyncRetrieverBuilder interface {
	Build() (GraphSyncRetriever, Cleanup, error)
}

type MockGraphSyncRetrieverBuilder struct {
	Retriever *MockGraphSyncRetriever
}

func (m *MockGraphSyncRetrieverBuilder) Build() (GraphSyncRetriever, Cleanup, error) {
	return m.Retriever, func() {}, nil
}

type GraphSyncRetrieverImpl struct {
	log      zerolog.Logger
	lotusAPI api.Gateway
	libp2p   *host.Host
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
	err = os.MkdirAll(tmpdir, 0755)
	if err != nil {
		return nil, nil, errors.Wrap(err, "failed to create temp folder")
	}
	retriever := GraphSyncRetrieverImpl{
		log:      log.With().Str("module", "retrieval_graphsync").Logger(),
		lotusAPI: g.LotusAPI,
		libp2p:   libp2p,
		tmpDir:   tmpdir,
	}
	return retriever, func() { os.RemoveAll(tmpdir) }, nil
}

type TimeEventPair struct {
	Timestamp time.Time `json:"timestamp"`
	Code      string    `json:"code"`
	Message   string    `json:"message"`
	Received  uint64    `json:"received"`
}

type retrievalStats struct {
	log    zerolog.Logger
	events []TimeEventPair
	done   chan interface{}
}

type CalculatedStats struct {
	Events             []TimeEventPair `json:"retrievalEvents,omitempty"`
	BytesDownloaded    uint64          `json:"bytesDownloaded,omitempty"`
	AverageSpeedPerSec float64         `json:"averageSpeedPerSec,omitempty"`
	TimeElapsed        time.Duration   `json:"timeElapsed,omitempty"`
	TimeToFirstByte    time.Duration   `json:"timeToFirstByte,omitempty"`
}

type ResultContent struct {
	Status       ResultStatus `json:"status"`
	ErrorMessage string       `json:"errorMessage,omitempty"`
	Protocol     Protocol     `json:"protocol"`
	CalculatedStats
}

func (r *retrievalStats) NewResultContent(status ResultStatus, errorMessage string) *ResultContent {
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
		if event.Code == string(rep.FirstByteCode) {
			firstByteTime = event.Timestamp
		}
	}
	var averageSpeedPerSec float64
	timeElapsedForDownload := lastEventTime.Sub(firstByteTime)
	if timeElapsedForDownload > 0 && bytesDownloaded > 0 {
		averageSpeedPerSec = float64(bytesDownloaded) / timeElapsedForDownload.Seconds()
	}
	var timeToFirstByte time.Duration
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
		Protocol: GraphSync,
	}
}

func (r *retrievalStats) OnRetrievalEvent(event rep.RetrievalEvent) {
	r.log.Info().Str("event_type", "retrieval_event").Str("code", string(event.Code())).Msg(string(event.Phase()))
	r.events = append(r.events, TimeEventPair{
		Timestamp: time.Now(),
		Code:      string(event.Code()),
		Message:   string(event.Phase()),
	})
	if event.Code() == rep.FailureCode || event.Code() == rep.SuccessCode {
		r.done <- event
	}
}

func (r *retrievalStats) OnDataTransferEvent(event datatransfer.Event, channelState datatransfer.ChannelState) {
	if event.Code == datatransfer.DataReceived && channelState.Received() == 0 {
		return
	}
	r.log.Info().
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
		datatransfer.RequestCancelled}
	r.events = append(r.events, TimeEventPair{
		Timestamp: event.Timestamp,
		Code:      datatransfer.Events[event.Code],
		Message:   event.Message,
		Received:  channelState.Received(),
	})
	if slices.Contains(errorEvents, event.Code) {
		r.done <- event
	}
}

func setupWallet(dir string) (*wallet.LocalWallet, error) {
	kstore, err := keystore.OpenOrInitKeystore(dir)
	if err != nil {
		return nil, err
	}

	wallet, err := wallet.NewWallet(kstore)
	if err != nil {
		return nil, err
	}

	addrs, err := wallet.WalletList(context.TODO())
	if err != nil {
		return nil, err
	}

	if len(addrs) == 0 {
		_, err := wallet.WalletNew(context.TODO(), types.KTBLS)
		if err != nil {
			return nil, err
		}
	}

	return wallet, nil
}

func (g GraphSyncRetrieverImpl) newFilClient(baseDir string) (*filclient.FilClient, error) {
	bstoreDatastore, err := flatfs.CreateOrOpen(filepath.Join(baseDir, "blockstore"), flatfs.NextToLast(3), false)
	if err != nil {
		return nil, errors.Wrap(err, "cannot create or open flatfs blockstore")
	}

	bstore := blockstore.NewBlockstoreNoPrefix(bstoreDatastore)
	if err != nil {
		return nil, errors.Wrap(err, "cannot create blockstore")
	}

	ds, err := levelds.NewDatastore(filepath.Join(baseDir, "datastore"), nil)
	if err != nil {
		return nil, errors.Wrap(err, "cannot create leveldb datastore")
	}

	wallet, err := setupWallet(filepath.Join(baseDir, "wallet"))
	if err != nil {
		return nil, errors.Wrap(err, "cannot create wallet")
	}

	addr, err := wallet.GetDefault()
	if err != nil {
		return nil, errors.Wrap(err, "cannot get default wallet address")
	}

	fc, err := filclient.NewClient(*g.libp2p, g.lotusAPI, wallet, addr, bstore, ds, baseDir)
	if err != nil {
		return nil, errors.Wrap(err, "cannot create filclient")
	}

	return fc, nil
}

func (g GraphSyncRetrieverImpl) Retrieve(parent context.Context, provider string, dataCid cid.Cid, timeout time.Duration) (*ResultContent, error) {
	minerInfoResult, err := module.GetMinerInfo(parent, g.lotusAPI, provider)
	if err != nil {
		return nil, errors.Wrap(err, "failed to get miner info")
	}

	if minerInfoResult.ErrorCode != "" {
		return &ResultContent{
			Status:       ResultStatus(minerInfoResult.ErrorCode),
			ErrorMessage: minerInfoResult.ErrorMessage,
		}, nil
	}

	ctx, cancel := context.WithTimeout(parent, timeout)
	defer cancel()

	filClient, err := g.newFilClient(g.tmpDir)
	if err != nil {
		return nil, errors.Wrap(err, "cannot create filClient")
	}

	stats := &retrievalStats{
		log:    g.log,
		done:   make(chan interface{}),
		events: make([]TimeEventPair, 0),
	}
	filClient.SubscribeToRetrievalEvents(stats)
	filClient.SubscribeToDataTransferEvents(stats.OnDataTransferEvent)
	go func() {
		query, err := filClient.RetrievalQuery(ctx, minerInfoResult.MinerAddress, dataCid)
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

		_, err = filClient.RetrieveContent(ctx, minerInfoResult.MinerAddress, proposal)
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
			} else {
				return stats.NewResultContent(RetrieveFailure, string(result.Phase())), nil
			}
		case datatransfer.Event:
			return stats.NewResultContent(DataTransferFailure, result.Message), nil
		default:
			return nil, errors.New("unknown result type")
		}
	}
}
