package indexprovider

import (
	"context"
	"validation-bot/module"
	"validation-bot/role"
	"validation-bot/task"

	"github.com/filecoin-project/go-legs/p2p/protocol/head"
	"github.com/filecoin-project/lotus/api"
	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/multiformats/go-multistream"
	"github.com/pkg/errors"
	"github.com/rs/zerolog"
	log2 "github.com/rs/zerolog/log"
)

type Dispatcher struct {
	module.SimpleDispatcher
}

func (d Dispatcher) Validate(definition task.Definition) error {
	return nil
}

type Auditor struct {
	log      zerolog.Logger
	lotusAPI api.Gateway
}

func NewAuditor(lotusAPI api.Gateway) Auditor {
	return Auditor{
		lotusAPI: lotusAPI,
		log:      log2.With().Str("role", "traceroute_auditor").Caller().Logger(),
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

	rootCid, err := head.QueryRootCid(ctx, libp2p, "/indexer/ingest/mainnet", *result.PeerID)
	if err != nil {
		if errors.Is(err, multistream.ErrNotSupported) {
			q.log.Info().Str("provider", provider).Err(err).Msg("index provider is not enabled")
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
