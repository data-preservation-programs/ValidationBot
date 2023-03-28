package retrieval

import (
	"context"
	"fmt"
	"time"
	"validation-bot/module"
	"validation-bot/task"

	"github.com/ipfs/go-blockservice"
	cid "github.com/ipfs/go-cid"
	"github.com/ipfs/go-datastore"
	blockstore "github.com/ipfs/go-ipfs-blockstore"
	blocks "github.com/ipfs/go-libipfs/blocks"
	"github.com/ipfs/go-merkledag"
	routinghelpers "github.com/libp2p/go-libp2p-routing-helpers"
	"github.com/libp2p/go-libp2p/core/host"
	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/libp2p/go-libp2p/core/peerstore"

	"github.com/pkg/errors"
	"github.com/rs/zerolog"
	log "github.com/rs/zerolog/log"

	bsclient "github.com/ipfs/go-libipfs/bitswap/client"
	bsmsg "github.com/ipfs/go-libipfs/bitswap/message"
	bsnet "github.com/ipfs/go-libipfs/bitswap/network"
)

type SessionTimeout string

const (
	completionTime = 15 * time.Second
)

// simple Libp2p interface to allow for testing (bypassing Connect on mocks).
type Libp2pish interface {
	Connect(ctx context.Context, addrInfo peer.AddrInfo) error
	Close() error
}

type BlockReader interface {
	GetBlock(ctx context.Context, c cid.Cid) (blocks.Block, error)
	Close() error
	ReceiveMessage(
		ctx context.Context,
		sender peer.ID,
		incoming bsmsg.BitSwapMessage)

	ReceiveError(error)

	// Connected/Disconnected warns bitswap about peer connections.
	PeerConnected(peer.ID)
	PeerDisconnected(peer.ID)
}

type BitswapRetriever struct {
	log          zerolog.Logger
	done         chan interface{}
	bitswap      BlockReader
	libp2p       Libp2pish
	peerInfo     MinerProtocols
	network      bsnet.BitSwapNetwork
	events       []TimeEventPair
	cidDurations map[cid.Cid]time.Duration
	startTime    time.Time
}

type BitswapRetrieverBuilder struct{}

func (b *BitswapRetrieverBuilder) Build(
	minerInfo *module.MinerInfoResult,
	protocol MinerProtocols,
	libp2p host.Host,
) (*BitswapRetriever, Cleanup, error) {
	// nolint:goconst
	if protocol.Protocol.Name != "bitswap" {
		return nil, nil, errors.New("protocol is not bitswap")
	}

	network := bsnet.NewFromIpfsHost(libp2p, routinghelpers.Null{})

	for _, addr := range protocol.MultiAddrs {
		if addr == nil {
			continue
		}
		maddr, err := peer.AddrInfoFromP2pAddr(addr)
		if err != nil {
			return nil, nil, errors.Wrap(err, fmt.Sprintf("cannot parse multiaddr %s", addr))
		}
		libp2p.Peerstore().AddAddrs(maddr.ID, maddr.Addrs, peerstore.PermanentAddrTTL)
	}

	ctx := context.WithValue(context.Background(), SessionTimeout("sessionTimeout"), completionTime)

	bswap := bsclient.New(ctx, network, blockstore.NewBlockstore(datastore.NewNullDatastore()))

	// nolint:exhaustruct
	return &BitswapRetriever{
			log:          log.With().Str("role", "retrieval_bitswap").Caller().Logger(),
			bitswap:      bswap,
			network:      network,
			done:         make(chan interface{}),
			events:       make([]TimeEventPair, 0),
			cidDurations: make(map[cid.Cid]time.Duration),
			startTime:    time.Time{},
			libp2p:       libp2p,
			peerInfo:     protocol,
		}, func() {
			libp2p.Close()
		}, nil
}

func (b *BitswapRetriever) Type() task.Type {
	return task.Retrieval
}

// nolint:lll
func (b *BitswapRetriever) Retrieve(ctx context.Context, root cid.Cid, timeout time.Duration) (
	*ResultContent,
	error,
) {
	b.network.Start(b.bitswap)

	addrInfo := peer.AddrInfo{ID: b.peerInfo.PeerID, Addrs: b.peerInfo.MultiAddrs}
	err := b.libp2p.Connect(ctx, addrInfo)
	if err != nil {
		return nil, errors.Wrap(err, "cannot connect to miner")
	}

	go func() {
		b.startTime = time.Now()
		var tout time.Duration

		if timeout > 0 {
			tout = timeout
		} else {
			tout = completionTime
		}

		ctx, cancel := context.WithDeadline(ctx, b.startTime.Add(tout))
		dserv := merkledag.NewReadOnlyDagService(merkledag.NewSession(ctx, merkledag.NewDAGService(blockservice.New(blockstore.NewBlockstore(datastore.NewNullDatastore()), b))))

		defer func() {
			defer cancel()
			b.bitswap.Close()
		}()

		linkGetter := merkledag.GetLinksWithDAG(dserv)
		visit := func(cid cid.Cid) bool {
			return true
		}

		err := merkledag.Walk(ctx, linkGetter, root, visit, merkledag.Concurrent())

		if errors.Is(err, ErrMaxTimeReached) {
			b.done <- ResultContent{
				Status:       Success,
				ErrorMessage: "",
			}
		}

		if err != nil {
			b.done <- ResultContent{
				Status:       RetrieveFailure,
				ErrorMessage: errors.Wrap(err, "failed while traversing").Error(),
			}
		}

		b.done <- ResultContent{
			Status:       Success,
			ErrorMessage: "",
		}
	}()

	select {
	case <-ctx.Done():
		return b.NewResultContent(RetrieveComplete, ""), nil
	case result := <-b.done:
		switch result := result.(type) {
		case ResultContent:
			return b.NewResultContent(result.Status, result.ErrorMessage), nil
		default:
			return nil, errors.New("unknown result type")
		}
	}
}

func (b *BitswapRetriever) NewResultContent(status ResultStatus, errorMessage string) *ResultContent {
	var timeToFirstByte time.Duration
	var totalDuration time.Duration
	var averageSpeedPerSec float64
	var size uint64

	for _, event := range b.events {
		size += event.Received

		if event.Code == string(FirstByteReceived) {
			timeToFirstByte = event.Timestamp.Sub(b.startTime)
			break
		}
	}

	for _, d := range b.cidDurations {
		totalDuration += d
	}

	if totalDuration != 0 {
		averageSpeedPerSec = float64(size) / float64(totalDuration)
	}

	return &ResultContent{
		Status:       status,
		ErrorMessage: errorMessage,
		Protocol:     Bitswap,
		CalculatedStats: CalculatedStats{
			Events:             b.events,
			BytesDownloaded:    size,
			AverageSpeedPerSec: averageSpeedPerSec,
			TimeElapsed:        totalDuration,
			TimeToFirstByte:    timeToFirstByte,
		},
	}
}

func (b *BitswapRetriever) Close() error {
	b.network.Stop()
	return nil
}

func (b *BitswapRetriever) GetBlock(ctx context.Context, c cid.Cid) (blocks.Block, error) {
	t0 := time.Now()
	blk, err := b.bitswap.GetBlock(ctx, c)
	b.cidDurations[c] = time.Since(t0)
	if err != nil {
		b.events = append(b.events,
			TimeEventPair{
				Timestamp: time.Now(),
				Code:      string(RetrieveFailure),
				Message:   fmt.Sprintf("failure: %s", c.String()),
				Received:  0,
			},
		)
		return nil, errors.Wrap(err, fmt.Sprintf("cannot get block [%s]", c.String()))
	}

	return blk, nil
}

func (b *BitswapRetriever) GetBlocks(context.Context, []cid.Cid) (<-chan blocks.Block, error) {
	return nil, errors.New("not implemented")
}

func (b *BitswapRetriever) NotifyNewBlocks(ctx context.Context, blks ...blocks.Block) error {
	for _, block := range blks {
		// nolint:exhaustruct
		event := TimeEventPair{
			Timestamp: time.Now(),
			Received:  uint64(len(block.RawData())),
		}

		if len(b.events) == 0 {
			event.Message = fmt.Sprintf("first-bytes received: [Block %s]", block.Cid())
			event.Code = string(FirstByteReceived)
		} else {
			event.Message = fmt.Sprintf("bytes received: [Block %s]", block.Cid())
			event.Code = string(BlockReceived)
		}

		b.events = append(b.events, event)
	}

	return nil
}
