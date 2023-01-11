package retrieval

import (
	"context"
	"fmt"
	"time"
	"validation-bot/module"
	"validation-bot/role"
	"validation-bot/task"

	cid "github.com/ipfs/go-cid"
	ipld "github.com/ipfs/go-ipld-format"
	"github.com/ipfs/go-merkledag"

	// ipld "github.com/ipld/go-ipld-prime"
	"github.com/libp2p/go-libp2p/core/host"
	"github.com/pkg/errors"
	"github.com/rs/zerolog"
	log "github.com/rs/zerolog/log"
	bswap "github.com/willscott/go-selfish-bitswap-client"
	"golang.org/x/sync/errgroup"
)

const (
	BITSWAP_PROTOCOL = "/ipfs/bitswap/1.2.0"
)

type traverser struct {
}
type BitswapRetriever struct {
	log     zerolog.Logger
	libp2p  host.Host
	tmpDir  string
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

func (b *BitswapRetriever) Retrieve(ctx context.Context, root cid.Cid) (*ResultContent, error) {
	queryContext, cancel := context.WithTimeout(ctx, time.Minute)
	defer cancel()

	go func() {
		<-queryContext.Done()
	}()

	t0 := time.Now()
	node, err := b.Get(queryContext, root)
	if err != nil {
		// TODO revisit this; err already has event
		return &ResultContent{
			Status:       RetrieveFailure,
			ErrorMessage: errors.Wrap(err, "failed to get block via Bitswap").Error(),
			Protocol:     Bitswap,
		}, nil
	}

	ttfb := time.Duration(time.Since(t0).Milliseconds())

	Walk(ctx, root, b)
	// stat, err := node.Stat()
	// if err != nil {
	// 	b.events = append(
	// 		b.events,
	// 		TimeEventPair{
	// 			Timestamp: time.Now(),
	// 			Code:      string(RetrieveFailure),
	// 			Message:   fmt.Sprintf("failed to get node stat: %s - %s", root, err),
	// 		},
	// 	)
	// 	return nil, errors.Wrap(err, "failed to get node stat")
	// }

	// b.GetMany(ctx, node)
	elapsed := time.Duration(time.Since(t0).Milliseconds())

	return &ResultContent{
		Status:   "success",
		Protocol: Bitswap,
		CalculatedStats: CalculatedStats{
			Events:             b.events,
			BytesDownloaded:    uint64(stat.CumulativeSize),
			AverageSpeedPerSec: float64(stat.CumulativeSize) / elapsed.Seconds(),
			TimeElapsed:        elapsed,
			TimeToFirstByte:    ttfb,
		},
	}, nil
}

func (b *BitswapRetriever) Get(ctx context.Context, root cid.Cid) (ipld.Node, error) {
	bytes, err := b.bitswap.Get(root)

	if err != nil {
		msg := fmt.Sprintf("failed to get block %s - %s", root, err)

		b.events = append(
			b.events,
			TimeEventPair{
				Timestamp: time.Now(),
				Code:      string(RetrieveFailure),
				Message:   msg,
			},
		)
		return nil, errors.Wrap(err, msg)
	}

	node := merkledag.NewRawNode(bytes)

	stat, err := node.Stat()
	if err != nil {
		b.events = append(
			b.events,
			TimeEventPair{
				Timestamp: time.Now(),
				Code:      string(RetrieveFailure),
				Message:   fmt.Sprintf("failed to get node stat: %s - %s", root, err),
			},
		)
		return nil, errors.Wrap(err, "failed to get node stat")
	}

	b.events = append(
		b.events,
		TimeEventPair{
			Timestamp: time.Now(),
			Code:      string(Success),
			Message:   fmt.Sprintf("got block %s", stat.Hash),
			Received:  uint64(stat.DataSize),
		},
	)

	return node, nil
}

func (b *BitswapRetriever) GetMany(ctx context.Context, nd ipld.Node) error {
	var cids []cid.Cid
	for _, link := range nd.Links() {
		cids = append(cids, link.Cid)
	}

	// traverse cids and call Get on each
	eg, ctx := errgroup.WithContext(ctx)

	for _, cid := range cids {
		cid := cid
		eg.Go(func() error {
			_, err := t.Get(ctx, cid)
			return err
		})
	}

	return nil
}

func Walk(ctx context.Context, c cid.Cid, ng ipld.NodeGetter) error {
	nd, err := ng.Get(ctx, c)
	if err != nil {
		return err
	}

	return walk(ctx, nd, ng)
}

func walk(ctx context.Context, nd ipld.Node, ng ipld.NodeGetter) error {
	var cids []cid.Cid
	for _, link := range nd.Links() {
		cids = append(cids, link.Cid)
	}

	eg, gctx := errgroup.WithContext(ctx)

	ndChan := ng.GetMany(ctx, cids)
	for ndOpt := range ndChan {
		if ndOpt.Err != nil {
			return ndOpt.Err
		}

		nd := ndOpt.Node
		eg.Go(func() error {
			return walk(gctx, nd, ng)
		})
	}

	err := eg.Wait()
	if err != nil {
		return err
	}

	return nil
}
