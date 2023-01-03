package retrieval

import (
	"context"
	"time"
	"validation-bot/module"
	"validation-bot/role"
	"validation-bot/task"

	"github.com/ipfs/go-cid"
	"github.com/ipld/go-ipld-prime/node/basicnode"
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
	tmpDir  string
	bitswap *bswap.Session
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

func (b *BitswapRetriever) Retrieve(ctx context.Context, cid cid.Cid) (*ResultContent, error) {
	queryContext, cancel := context.WithTimeout(ctx, time.Minute)
	defer cancel()

	go func() {
		<-queryContext.Done()
	}()

	t0 := time.Now()
	root, err := b.bitswap.Get(cid)
	elapsed := time.Duration(time.Since(t0).Milliseconds())

	basicnode.Prototype.Bytes.NewBuilder().AssignBytes(root)

	if err != nil {
		return &ResultContent{
			Status:       RetrieveFailure,
			ErrorMessage: errors.Wrap(err, "failed to get block via Bitswap").Error(),
			Protocol:     Bitswap,
		}, nil
	}

	return &ResultContent{
		Status:   "success",
		Protocol: Bitswap,
		CalculatedStats: CalculatedStats{
			Events: []TimeEventPair{
				TimeEventPair{
					Timestamp: time.Now(),
					Code:      string(Success),
					Message:   "start",
					Received:  root,
				},
			},
			BytesDownloaded:    len(uint64(root)),
			AverageSpeedPerSec: float64(len(root)) / elapsed.Seconds(),
			TimeElapsed:        elapsed,
			TimeToFirstByte:    elapsed,
		},
	}, nil
}
