package retrieval

import (
	"context"
	"fmt"
	"time"
	"validation-bot/module"
	"validation-bot/role"
	"validation-bot/task"

	cid "github.com/ipfs/go-cid"

	bswap "github.com/brossetti1/go-selfish-bitswap-client"
	gocar "github.com/ipld/go-car"
	selectorparse "github.com/ipld/go-ipld-prime/traversal/selector/parse"
	"github.com/pkg/errors"
	"github.com/rs/zerolog"
	log "github.com/rs/zerolog/log"
)

const (
	completionTime = 20 * time.Second
)

var ErrMaxTimeReached = errors.New("dump session complete")

type BitswapRetriever struct {
	log          zerolog.Logger
	done         chan interface{}
	bitswap      bswap.Bitswap
	events       []TimeEventPair
	cidDurations map[cid.Cid]time.Duration
	size         uint64
	startTime    time.Time
}

type BitswapRetrieverBuilder struct{}

// github.com/willscott/go-selfish-bitswap-client has a hardcoded timeout for Session
// of 10 seconds. Because of this, we need to make a new session for each block if we want
// the Dump process to take longer than 10 seconds in total. This is accomplished by using
// the bitswapCallback function to create a new session for each block.
func (b *BitswapRetrieverBuilder) Build(
	ctx context.Context,
	minerInfo *module.MinerInfoResult,
) (*BitswapRetriever, Cleanup, error) {
	libp2p, err := role.NewLibp2pHostWithRandomIdentityAndPort()
	if err != nil {
		return nil, nil, errors.Wrap(err, "cannot create libp2p host")
	}

	fmt.Printf("minerInfo: %v\n", minerInfo)

	opts := bswap.Options{
		SessionTimeout: completionTime,
	}

	// nolint:exhaustruct
	return &BitswapRetriever{
			log:          log.With().Str("role", "retrieval_bitswap").Caller().Logger(),
			bitswap:      bswap.New(libp2p, *minerInfo.PeerID, opts),
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

// callback gets called after a successful block retrieval during traverser.Dump.
func (b *BitswapRetriever) onNewBlock(block Block) {
	// nolint:exhaustruct
	event := TimeEventPair{
		Timestamp: time.Now(),
		Received:  block.Size,
	}

	if len(b.events) == 0 {
		event.Message = fmt.Sprintf("first-bytes received: [Block %s]", block.BlockCID)
		event.Code = string(FirstByteReceived)
	} else {
		event.Message = fmt.Sprintf("bytes received: [Block %s]", block.BlockCID)
		event.Code = string(BlockReceived)
	}

	b.events = append(b.events, event)
}

// nolint:lll
func (b *BitswapRetriever) Retrieve(ctx context.Context, root cid.Cid, timeout time.Duration) (
	*ResultContent,
	error,
) {
	dag := gocar.Dag{Root: root, Selector: selectorparse.CommonSelector_ExploreAllRecursively}

	traverser, err := NewTraverser(b, []gocar.Dag{dag})
	if err != nil {
		return nil, errors.Wrap(err, "cannot prepare Selector")
	}

	defer b.bitswap.Close()

	go func() {
		b.startTime = time.Now()
		var tout time.Duration

		if timeout > 0 {
			tout = timeout
		} else {
			tout = completionTime
		}

		ctx, cancel := context.WithDeadline(ctx, b.startTime.Add(tout))
		defer cancel()

		err = traverser.traverse(ctx)

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
		b.size = traverser.Size()
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
			break
		}
	}

	for _, d := range b.cidDurations {
		totalDuration += d
	}

	if totalDuration != 0 {
		averageSpeedPerSec = float64(b.size) / float64(totalDuration)
	}

	return &ResultContent{
		Status:       status,
		ErrorMessage: errorMessage,
		Protocol:     Bitswap,
		CalculatedStats: CalculatedStats{
			Events:             b.events,
			BytesDownloaded:    b.size,
			AverageSpeedPerSec: averageSpeedPerSec,
			TimeElapsed:        totalDuration,
			TimeToFirstByte:    timeToFirstByte,
		},
	}
}

// Get matches the gocar.ReadStore interface used when traversing a car file.
// Because we initialize the bitswap session for each Get request, we meassure
// the duration of the request from the time the go-selfish-bitswap-client session
// initializes until when the block was received.
func (b *BitswapRetriever) Get(ctx context.Context, c cid.Cid) ([]byte, error) {
	t0 := time.Now()
	bytes, err := b.bitswap.Get(ctx, c)
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

	return bytes, nil
}
