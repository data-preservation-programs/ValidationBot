package indexprovider

import (
	"context"
	"path"

	"validation-bot/task"

	"validation-bot/module"
	"validation-bot/role"

	"github.com/filecoin-project/lotus/api"
	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/libp2p/go-libp2p/core/protocol"
	"github.com/multiformats/go-multistream"
	"github.com/pkg/errors"
	"github.com/rs/zerolog"
	log2 "github.com/rs/zerolog/log"
)

type Dispatcher struct {
	module.SimpleDispatcher
	module.NoopValidator
}

func (Dispatcher) Type() task.Type {
	return task.IndexProvider
}

type Auditor struct {
	log      zerolog.Logger
	lotusAPI api.Gateway
}

func (Auditor) Type() task.Type {
	return task.IndexProvider
}

func NewAuditor(lotusAPI api.Gateway) Auditor {
	return Auditor{
		lotusAPI: lotusAPI,
		log:      log2.With().Str("role", "indexprovider_auditor").Caller().Logger(),
	}
}

func (q Auditor) Validate(ctx context.Context, input module.ValidationInput) (*module.ValidationResult, error) {
	provider := input.Target
	log := q.log.With().Str("provider", provider).Logger()

	log.Info().Msg("start validating index provider status")

	result, err := q.Enabled(ctx, provider)
	if err != nil {
		return nil, errors.Wrap(err, "failed to validate provider")
	}

	jsonb, err := module.NewJSONB(result)
	if err != nil {
		return nil, errors.Wrap(err, "failed to marshal result")
	}

	return &module.ValidationResult{
		ValidationInput: input,
		Result:          jsonb,
	}, nil
}

func (q Auditor) Enabled(ctx context.Context, provider string) (*ResultContent, error) {
	result, err := module.GetMinerInfo(ctx, q.lotusAPI, provider)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to get miner info for %s", provider)
	}

	if result.ErrorCode != "" {
		return &ResultContent{
			Status:       Status(result.ErrorCode),
			ErrorMessage: result.ErrorMessage,
		}, nil
	}

	if len(result.MultiAddrs) == 0 {
		return &ResultContent{
			Status:       NoMultiAddress,
			ErrorMessage: "miner has no multi address",
		}, nil
	}

	libp2p, err := role.NewLibp2pHostWithRandomIdentityAndPort()
	if err != nil {
		return nil, errors.Wrap(err, "failed to create libp2p host")
	}

	defer libp2p.Close()

	err = libp2p.Connect(
		ctx, peer.AddrInfo{
			ID:    *result.PeerID,
			Addrs: result.MultiAddrs,
		},
	)
	if err != nil {
		//nolint:nilerr
		return &ResultContent{
			Status:       CannotConnect,
			ErrorMessage: err.Error(),
		}, nil
	}

	topic := "/indexer/ingest/mainnet"

	rootCid, err := QueryRootCid(ctx, libp2p, topic, *result.PeerID)
	if err != nil {
		protocolID := protocol.ID(path.Join("/legs/head", topic, "0.0.1"))

		if !errors.Is(err, multistream.ErrNotSupported[protocol.ID]{Protos: []protocol.ID{protocolID}}) {
			q.log.Info().Str("provider", provider).AnErr("err", err).Msg("index provider is not enabled")
			return &ResultContent{
				Status: Disabled,
			}, nil
		}

		return &ResultContent{
			Status:       CannotConnect,
			ErrorMessage: err.Error(),
		}, nil
	}

	q.log.Info().Str("provider", provider).
		Str("rootCid", rootCid.String()).Msg("got rootCid from index provider")

	return &ResultContent{
		Status:  Enabled,
		RootCid: rootCid.String(),
	}, nil
}

func (q Auditor) ShouldValidate(ctx context.Context, input module.ValidationInput) (bool, error) {
	return true, nil
}
