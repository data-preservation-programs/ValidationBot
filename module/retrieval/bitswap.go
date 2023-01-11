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
	"github.com/pkg/errors"
	"github.com/rs/zerolog"
	log "github.com/rs/zerolog/log"
	bswap "github.com/willscott/go-selfish-bitswap-client"
)

const (
	retrievalTimeout = 1 * time.Minute
)

// return interface of get
type BitswapRetriever struct {
	log          zerolog.Logger
	done         chan interface{}
	bitswap      func() *bswap.Session
	events       []TimeEventPair
	cidDurations map[cid.Cid]time.Duration
	traverser    gocar.SelectiveCarPrepared
	startTime    time.Time
}

type BitswapRetrieverBuilder struct{}

func (b *BitswapRetrieverBuilder) Build(
	ctx context.Context,
	minerInfo *module.MinerInfoResult,
) (*BitswapRetriever, Cleanup, error) {
	libp2p, err := role.NewLibp2pHostWithRandomIdentityAndPort()
	if err != nil {
		return nil, nil, errors.Wrap(err, "cannot create libp2p host")
	}

	bitswapCallback := func() *bswap.Session { return bswap.New(libp2p, *minerInfo.PeerID) }

	return &BitswapRetriever{
			log:          log.With().Str("role", "retrieval_bitswap").Caller().Logger(),
			bitswap:      bitswapCallback,
			done:         make(chan interface{}),
			events:       make([]TimeEventPair, 0),
			cidDurations: make(map[cid.Cid]time.Duration),
			startTime:    time.Time{},
		}, func() {
			libp2p.Close()
		}, nil
}

func (b *BitswapRetriever) Type() task.Type {
	return task.Retrieval
}

func (b *BitswapRetriever) Retrieve(ctx context.Context, root cid.Cid, timeout time.Duration) (*ResultContent, error) {
	dag := gocar.Dag{Root: root, Selector: selectorparse.CommonSelector_ExploreAllRecursively}

	// TODO: could potentially use gocar.MaxTraversalLinks(int) to cap this?
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
		} else {
			b.events = append(b.events,
				TimeEventPair{
					Timestamp: t,
					Code:      string(BlockReceived),
					Message:   fmt.Sprintf("bytes received: [Block %s]", block.BlockCID),
					Received:  block.Size,
				},
			)
		}

		return nil
	}

	// im actually not sure if the Prepare or the Dump method
	// calls the miner and traverses the CIDs?
	traverser, err := car.Prepare(onNewCarBlock)
	if err != nil {
		return nil, errors.Wrap(err, "cannot prepare Selector")
	}

	b.traverser = traverser

	ctx, cancel := context.WithDeadline(ctx, b.startTime.Add(retrievalTimeout))
	defer cancel()

	go func() {
		b.startTime = time.Now()
		err = b.traverser.Dump(ctx, io.Discard)
		if err != nil {
			b.done <- ResultContent{
				Status:       RetrieveFailure,
				ErrorMessage: errors.Wrap(err, "failed while traversing").Error(),
			}
		}

		b.done <- ResultContent{
			Status:       RetrieveFailure,
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

	for _, event := range b.events {
		if event.Code == string(FirstByteReceived) {
			timeToFirstByte = event.Timestamp.Sub(b.startTime)
		}
	}

	for _, d := range b.cidDurations {
		totalDuration += d
	}

	if totalDuration != 0 {
		averageSpeedPerSec = float64(b.traverser.Size()) / float64(totalDuration)
	}

	return &ResultContent{
		Status:       status,
		ErrorMessage: errorMessage,
		Protocol:     Bitswap,
		CalculatedStats: CalculatedStats{
			Events:             b.events,
			BytesDownloaded:    b.traverser.Size(),
			AverageSpeedPerSec: averageSpeedPerSec,
			TimeElapsed:        totalDuration,
			TimeToFirstByte:    timeToFirstByte,
		},
	}
}

// Get matches the gocar.ReadStore interface used when traversing a car file.
func (b *BitswapRetriever) Get(ctx context.Context, c cid.Cid) (blocks.Block, error) {
	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	default:
	}

	t0 := time.Now()
	session := b.bitswap()
	defer func() {
		b.cidDurations[c] = time.Since(t0)
		session.Close()
	}()

	bytes, err := session.Get(c)
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

	return blocks.NewBlockWithCid(bytes, c)
}
