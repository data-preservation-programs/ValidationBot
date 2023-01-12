package retrieval

import (
	"context"
	"fmt"
	"io"
	"time"
	"validation-bot/module"
	"validation-bot/role"
	"validation-bot/task"

	cid "github.com/ipfs/go-cid"

	blocks "github.com/ipfs/go-block-format"
	gocar "github.com/ipld/go-car"
	selectorparse "github.com/ipld/go-ipld-prime/traversal/selector/parse"
	"github.com/libp2p/go-libp2p/core/host"
	"github.com/pkg/errors"
	"github.com/rs/zerolog"
	log "github.com/rs/zerolog/log"
	bswap "github.com/willscott/go-selfish-bitswap-client"
)

const (
	BITSWAP_PROTOCOL = "/ipfs/bitswap/1.2.0"
)

type BitswapRetriever struct {
	log     zerolog.Logger
	libp2p  host.Host
	bitswap *bswap.Session
	events  []TimeEventPair
}

type BitswapRetrieverBuilder struct{}

func (b *BitswapRetrieverBuilder) Build(ctx context.Context, minerInfo *module.MinerInfoResult) (*BitswapRetriever, Cleanup, error) {
	libp2p, err := role.NewLibp2pHostWithRandomIdentityAndPort()
	if err != nil {
		return nil, nil, errors.Wrap(err, "cannot create libp2p host")
	}

	session := bswap.New(libp2p, *minerInfo.PeerID)

	return &BitswapRetriever{
			log:     log.With().Str("role", "retrieval_bitswap").Caller().Logger(),
			libp2p:  libp2p,
			bitswap: session,
		}, func() {
			libp2p.Close()
		}, nil
}

func (b *BitswapRetriever) Type() task.Type {
	return task.Retrieval
}

func (b *BitswapRetriever) Retrieve(ctx context.Context, root cid.Cid, timeout time.Duration) (*ResultContent, error) {
	dag := gocar.Dag{Root: root, Selector: selectorparse.CommonSelector_ExploreAllRecursively}

	/*
	 * TODO:
	 * could potentially use MaxTraversalLinks to cap this?
	 *   option: gocar.MaxTraversalLinks(int) changes the allowed number
	 *   of links a selector traversal can execute before failing.
	 * otherwise, we need a timeout for the whole traversal
	 */
	car := gocar.NewSelectiveCar(ctx, b, []gocar.Dag{dag}, gocar.TraverseLinksOnlyOnce())

	onNewCarBlock := func(block gocar.Block) error {
		t := time.Now()

		if len(b.events) == 0 {
			b.events = append(b.events,
				TimeEventPair{
					Timestamp: t,
					Code:      string(FirstByteReceived),
					Message:   fmt.Sprintf("first-bytes received: [Block %s]", block.BlockCID),
					Received:  block.Size,
				},
			)
		}

		b.events = append(b.events,
			TimeEventPair{
				Timestamp: t,
				Code:      string(BlockReceived),
				Message:   fmt.Sprintf("bytes received: [Block %s]", block.BlockCID),
				Received:  block.Size,
			},
		)

		return nil
	}

	prepared, err := car.Prepare(onNewCarBlock)
	if err != nil {
		return nil, errors.Wrap(err, "cannot prepare Selector")
	}

	t0 := time.Now()
	ctx, cancel := context.WithDeadline(ctx, t0.Add(10*time.Second))
	defer cancel()

	err = prepared.Dump(ctx, io.Discard)
	if err != nil {
		return nil, errors.Wrap(err, "cannot dump car")
	}

	// TODO: time elapsed or timeout? ie 10 seconds?
	timeElapsed := time.Since(t0)
	var timeToFirstByte time.Duration

	for _, event := range b.events {
		if event.Code == string(FirstByteReceived) {
			timeToFirstByte = event.Timestamp.Sub(t0)
		}
	}

	return &ResultContent{
		Status:   Success,
		Protocol: Bitswap,
		CalculatedStats: CalculatedStats{
			Events:             b.events,
			BytesDownloaded:    prepared.Size(),
			AverageSpeedPerSec: float64(prepared.Size()) / timeElapsed.Seconds(),
			TimeElapsed:        timeElapsed,
			TimeToFirstByte:    timeToFirstByte,
		},
	}, nil
}

func (b *BitswapRetriever) Get(ctx context.Context, c cid.Cid) (blocks.Block, error) {
	bytes, err := b.bitswap.Get(c)

	/*
	 * TODO: more descriptive error to catch after
	 *   err = prepared.Dump(ctx, io.Discard) and return ResultContent
	 */
	if err != nil {
		b.events = append(b.events,
			TimeEventPair{
				Timestamp: time.Now(),
				Code:      string(RetrieveFailure),
				Message:   fmt.Sprintf("failure: %s", c.String()),
			},
		)
		return nil, errors.Wrap(err, "cannot get block")
	}

	return blocks.NewBlockWithCid(bytes, c)
}
