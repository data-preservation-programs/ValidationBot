package queryask

import (
	"context"
	"time"

	"validation-bot/role"
	"validation-bot/task"

	"validation-bot/module"

	cborutil "github.com/filecoin-project/go-cbor-util"
	"github.com/filecoin-project/go-fil-markets/storagemarket/network"
	"github.com/filecoin-project/lotus/api"
	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/pkg/errors"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
)

type Dispatcher struct {
	module.SimpleDispatcher
	module.NoopValidator
}

func (Dispatcher) Type() task.Type {
	return task.QueryAsk
}

type Auditor struct {
	log      zerolog.Logger
	lotusAPI api.Gateway
}

func (Auditor) Type() task.Type {
	return task.QueryAsk
}

func NewAuditor(lotusAPI api.Gateway) Auditor {
	return Auditor{
		log:      log.With().Str("role", "query_ask_auditor").Caller().Logger(),
		lotusAPI: lotusAPI,
	}
}

func (q Auditor) Validate(ctx context.Context, input module.ValidationInput) (*module.ValidationResult, error) {
	q.log.Info().Str("target", input.Target).Msg("start validating query ask")
	provider := input.Target

	queryContext, cancel := context.WithTimeout(ctx, time.Minute)
	defer cancel()

	result, err := q.QueryMiner(queryContext, provider)
	if err != nil {
		return nil, errors.Wrap(err, "failed to query miner")
	}

	jsonb, err := module.NewJSONB(result)
	if err != nil {
		return nil, errors.Wrap(err, "failed to marshal result")
	}

	// TODO ipld here?
	return &module.ValidationResult{
		ValidationInput: input,
		Result:          jsonb,
	}, nil
}

//nolint:nilerr,funlen,cyclop
func (q Auditor) QueryMiner(ctx context.Context, provider string) (*ResultContent, error) {
	libp2p, err := role.NewLibp2pHostWithRandomIdentityAndPort()
	if err != nil {
		return nil, errors.Wrap(err, "failed to create libp2p host")
	}

	defer libp2p.Close()

	q.log.Debug().Str("provider", provider).Msg("querying miner info")

	minerInfoResult, err := module.GetMinerInfo(ctx, q.lotusAPI, provider)
	if err != nil {
		return nil, errors.Wrap(err, "failed to get miner info")
	}

	if minerInfoResult.ErrorCode != "" {
		return &ResultContent{
			Status:       QueryStatus(minerInfoResult.ErrorCode),
			ErrorMessage: minerInfoResult.ErrorMessage,
		}, nil
	}

	if len(minerInfoResult.MultiAddrs) == 0 {
		return &ResultContent{
			Status:       NoMultiAddress,
			ErrorMessage: "miner has no multi address",
		}, nil
	}

	addrInfo := peer.AddrInfo{
		ID:    *minerInfoResult.PeerID,
		Addrs: minerInfoResult.MultiAddrs,
	}
	now := time.Now()
	err = libp2p.Connect(ctx, addrInfo)

	if err != nil {
		return &ResultContent{
			Status:       CannotConnect,
			ErrorMessage: err.Error(),
		}, nil
	}

	q.log.Debug().Str("provider", provider).Msg("sending stream to /fil/storage/ask/1.1.0")

	stream, err := libp2p.NewStream(ctx, addrInfo.ID, "/fil/storage/ask/1.1.0")
	if err != nil {
		return &ResultContent{
			Status:       StreamFailure,
			ErrorMessage: err.Error(),
		}, nil
	}

	libp2p.ConnManager().Protect(stream.Conn().RemotePeer(), "GetAsk")

	defer func() {
		libp2p.ConnManager().Unprotect(stream.Conn().RemotePeer(), "GetAsk")
		stream.Close()
	}()

	askRequest := &network.AskRequest{Miner: minerInfoResult.MinerAddress}

	deadline, ok := ctx.Deadline()
	if ok {
		err = stream.SetDeadline(deadline)
		if err != nil {
			return nil, errors.Wrap(err, "failed to set deadline")
		}

		//nolint:errcheck
		defer stream.SetDeadline(time.Time{})
	}

	err = cborutil.WriteCborRPC(stream, askRequest)
	if err != nil {
		return &ResultContent{
			Status:       StreamFailure,
			ErrorMessage: err.Error(),
		}, nil
	}

	var resp network.AskResponse

	err = cborutil.ReadCborRPC(stream, &resp)
	if err != nil {
		return &ResultContent{
			Status:       StreamFailure,
			ErrorMessage: err.Error(),
		}, nil
	}

	latency := time.Since(now)

	return &ResultContent{
		PeerID:        minerInfoResult.PeerID.String(),
		MultiAddrs:    minerInfoResult.MultiAddrStr,
		Status:        Success,
		ErrorMessage:  "",
		Price:         resp.Ask.Ask.Price.String(),
		VerifiedPrice: resp.Ask.Ask.VerifiedPrice.String(),
		MinPieceSize:  uint64(resp.Ask.Ask.MinPieceSize),
		MaxPieceSize:  uint64(resp.Ask.Ask.MaxPieceSize),
		Latency:       latency,
	}, nil
}

func (q Auditor) ShouldValidate(ctx context.Context, input module.ValidationInput) (bool, error) {
	return true, nil
}
