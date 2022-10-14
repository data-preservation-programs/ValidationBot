package queryask

import (
	"context"
	"encoding/json"
	"reflect"
	"time"
	"validation-bot/task"

	"github.com/filecoin-project/go-address"
	cborutil "github.com/filecoin-project/go-cbor-util"
	"github.com/filecoin-project/go-fil-markets/storagemarket/network"
	"github.com/filecoin-project/lotus/api/v0api"
	"github.com/filecoin-project/lotus/chain/types"
	"github.com/libp2p/go-libp2p/core/host"
	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/multiformats/go-multiaddr"
	"github.com/pkg/errors"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
	"gorm.io/gorm"
)

type Input struct {
	task.Task
}

type QueryStatus string

const (
	Success             QueryStatus = "success"
	InvalidProviderId   QueryStatus = "invalid_provider_id"
	NoPeerId            QueryStatus = "no_peer_id"
	InvalidMultiAddress QueryStatus = "invalid_multi_address"
	CannotConnect       QueryStatus = "cannot_connect"
	NoMultiAddress      QueryStatus = "no_multi_address"
	StreamFailure       QueryStatus = "stream_failure"
)

type ResultContent struct {
	PeerId        string      `json:"peer_id,omitempty"`
	MultiAddrs    []string    `json:"multi_addrs,omitempty"`
	Status        QueryStatus `json:"status"`
	ErrorMessage  string      `json:"error_message,omitempty"`
	Price         string      `json:"price,omitempty"`
	VerifiedPrice string      `json:"verified_price,omitempty"`
	MinPieceSize  uint64      `json:"min_piece_size,omitempty"`
	MaxPieceSize  uint64      `json:"max_piece_size,omitempty"`
}

type Result struct {
	task.Task
	ResultContent
}

type ResultModel struct {
	gorm.Model
	Result
}

func (ResultModel) TableName() string {
	return "query_ask_results"
}

type QueryAsk struct {
	log      zerolog.Logger
	lotusApi v0api.Gateway
	libp2p   *host.Host
}

func NewQueryAskModule(libp2p *host.Host, lotusApi v0api.Gateway) QueryAsk {
	return QueryAsk{
		log:      log.With().Str("role", "query_ask_module").Logger(),
		libp2p:   libp2p,
		lotusApi: lotusApi,
	}
}

func (q QueryAsk) TaskType() task.Type {
	return task.QueryAsk
}

func (q QueryAsk) ResultType() interface{} {
	return &ResultModel{}
}

func (q QueryAsk) GetTasks(definitions []task.Definition) (map[task.Definition][]byte, error) {
	result := make(map[task.Definition][]byte)
	for _, definition := range definitions {
		input, _ := q.GetTask(definition)
		result[definition] = input
	}

	return result, nil
}

func (q QueryAsk) GetTask(definition task.Definition) ([]byte, error) {
	input := Input{
		Task: task.Task{
			Type:         definition.Type,
			DefinitionID: definition.ID,
			Target:       definition.Target,
		},
	}
	return json.Marshal(input)
}

func (q QueryAsk) Validate(ctx context.Context, input []byte) ([]byte, error) {
	var in Input

	err := json.Unmarshal(input, &in)
	if err != nil {
		return nil, errors.Wrap(err, "failed to unmarshal input")
	}

	provider := in.Target
	result, err := q.QueryMiner(ctx, provider)
	if err != nil {
		return nil, errors.Wrap(err, "failed to query miner")
	}

	return json.Marshal(result)
}

func (q QueryAsk) QueryMiner(ctx context.Context, provider string) (*ResultContent, error) {
	providerAddr, err := address.NewFromString(provider)
	if err != nil {
		return &ResultContent{
			Status:       InvalidProviderId,
			ErrorMessage: err.Error(),
		}, nil
	}
	minerInfo, err := q.lotusApi.StateMinerInfo(ctx, providerAddr, types.EmptyTSK)
	if err != nil {
		tp := reflect.TypeOf(err)
		if tp.String() == "*jsonrpc.respError" {
			return &ResultContent{
				Status:       InvalidProviderId,
				ErrorMessage: err.Error(),
			}, nil
		}
		return nil, err
	}

	if minerInfo.PeerId == nil {
		return &ResultContent{
			Status: NoPeerId,
		}, nil
	}

	var maddrs []multiaddr.Multiaddr
	var maddrStrs []string
	for _, mma := range minerInfo.Multiaddrs {
		ma, err := multiaddr.NewMultiaddrBytes(mma)
		if err != nil {
			return &ResultContent{
				Status:       InvalidMultiAddress,
				ErrorMessage: err.Error(),
			}, nil
		}
		maddrs = append(maddrs, ma)
		maddrStrs = append(maddrStrs, ma.String())
	}

	if len(maddrs) == 0 {
		return &ResultContent{
			Status: NoMultiAddress,
		}, nil
	}

	addrInfo := peer.AddrInfo{
		ID:    *minerInfo.PeerId,
		Addrs: maddrs,
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

	askRequest := &network.AskRequest{Miner: providerAddr}
	var resp network.AskResponse
	deadline, ok := ctx.Deadline()
	if ok {
		stream.SetDeadline(deadline)
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
		PeerId:        minerInfo.PeerId.String(),
		MultiAddrs:    maddrStrs,
		Status:        Success,
		ErrorMessage:  "",
		Price:         resp.Ask.Ask.Price.String(),
		VerifiedPrice: resp.Ask.Ask.VerifiedPrice.String(),
		MinPieceSize:  uint64(resp.Ask.Ask.MinPieceSize),
		MaxPieceSize:  uint64(resp.Ask.Ask.MaxPieceSize),
	}, nil
}
