package queryask

import (
	"context"
	"time"
	"validation-bot/module"

	cborutil "github.com/filecoin-project/go-cbor-util"
	"github.com/filecoin-project/go-fil-markets/storagemarket/network"
	"github.com/filecoin-project/lotus/api"
	"github.com/libp2p/go-libp2p/core/host"
	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/pkg/errors"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
)

type Dispatcher struct {
	module.SimpleDispatcher
}

type Auditor struct {
	log      zerolog.Logger
	lotusAPI api.Gateway
	libp2p   *host.Host
}

func NewAuditor(libp2p *host.Host, lotusAPI api.Gateway) Auditor {
	return Auditor{
		log:      log.With().Str("role", "query_ask_auditor").Logger(),
		libp2p:   libp2p,
		lotusAPI: lotusAPI,
	}
}

func (q Auditor) Validate(ctx context.Context, input module.ValidationInput) (*module.ValidationResult, error) {
	provider := input.Target
	result, err := q.QueryMiner(ctx, provider)
	if err != nil {
		return nil, errors.Wrap(err, "failed to query miner")
	}

	jsonb, err := module.NewJSONB(result)
	if err != nil {
		return nil, errors.Wrap(err, "failed to marshal result")
	}

	return &module.ValidationResult{
		Task:   input.Task,
		Result: jsonb,
	}, nil
}

//nolint:nilerr,funlen,cyclop
func (q Auditor) QueryMiner(ctx context.Context, provider string) (*ResultContent, error) {
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

	addrInfo := peer.AddrInfo{
		ID:    *minerInfoResult.PeerID,
		Addrs: minerInfoResult.MultiAddrs,
	}
	err = (*q.libp2p).Connect(ctx, addrInfo)

	if err != nil {
		return &ResultContent{
			Status:       CannotConnect,
			ErrorMessage: err.Error(),
		}, nil
	}

	stream, err := (*q.libp2p).NewStream(ctx, addrInfo.ID, "/fil/storage/ask/1.1.0")
	if err != nil {
		return &ResultContent{
			Status:       StreamFailure,
			ErrorMessage: err.Error(),
		}, nil
	}

	(*q.libp2p).ConnManager().Protect(stream.Conn().RemotePeer(), "GetAsk")
	defer func() {
		(*q.libp2p).ConnManager().Unprotect(stream.Conn().RemotePeer(), "GetAsk")
		stream.Close()
	}()

	askRequest := &network.AskRequest{Miner: minerInfoResult.MinerAddress}
	var resp network.AskResponse
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

	err = cborutil.ReadCborRPC(stream, &resp)
	if err != nil {
		return &ResultContent{
			Status:       StreamFailure,
			ErrorMessage: err.Error(),
		}, nil
	}

	return &ResultContent{
		PeerID:        minerInfoResult.PeerID.String(),
		MultiAddrs:    minerInfoResult.MultiAddrStr,
		Status:        Success,
		ErrorMessage:  "",
		Price:         resp.Ask.Ask.Price.String(),
		VerifiedPrice: resp.Ask.Ask.VerifiedPrice.String(),
		MinPieceSize:  uint64(resp.Ask.Ask.MinPieceSize),
		MaxPieceSize:  uint64(resp.Ask.Ask.MaxPieceSize),
	}, nil
}
