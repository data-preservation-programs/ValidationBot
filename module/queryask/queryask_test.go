package queryask

import (
	"context"
	"testing"

	"validation-bot/module"
	"validation-bot/role"
	"validation-bot/task"

	"github.com/filecoin-project/go-state-types/abi"
	"github.com/filecoin-project/lotus/api"
	"github.com/filecoin-project/lotus/api/client"
	"github.com/jackc/pgtype"
	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

func getModule(t *testing.T, mockGateway *module.MockGateway) Auditor {
	assert := assert.New(t)
	priv, _, _, err := role.GenerateNewPeer()
	assert.Nil(err)
	libp2p, err := role.NewLibp2pHost(priv, "/ip4/0.0.0.0/tcp/0")
	assert.Nil(err)
	assert.NotNil(libp2p)
	ctx := context.Background()
	var queryAsk Auditor
	if mockGateway == nil {
		lotusApi, closer, err := client.NewGatewayRPCV1(ctx, "https://api.node.glif.io/", nil)
		defer closer()
		assert.Nil(err)
		queryAsk = NewAuditor(libp2p, lotusApi)
	} else {
		queryAsk = NewAuditor(libp2p, mockGateway)
	}
	return queryAsk
}

func TestQueryAsk_QueryMiner_InvalidProviderId(t *testing.T) {
	assert := assert.New(t)
	queryAsk := getModule(t, nil)
	assert.NotNil(queryAsk)
	ctx := context.Background()
	result, err := queryAsk.QueryMiner(ctx, "xxxxxx")
	assert.Nil(err)
	assert.Equal(result.Status, InvalidProviderID)
	assert.Equal(result.ErrorMessage, "unknown address network")
}

func TestQueryAsk_QueryMiner_NotMinerAddress(t *testing.T) {
	assert := assert.New(t)
	queryAsk := getModule(t, nil)
	assert.NotNil(queryAsk)
	ctx := context.Background()
	result, err := queryAsk.QueryMiner(ctx, "f010000")
	assert.Nil(err)
	assert.Equal(result.Status, InvalidProviderID)
	assert.Equal(result.ErrorMessage, "failed to load miner actor state: actor code is not miner: paymentchannel")
}

func TestQueryAsk_QueryMiner_NoPeerId(t *testing.T) {
	assert := assert.New(t)
	mockGateway := new(module.MockGateway)
	mockGateway.On("StateMinerInfo", mock.Anything, mock.Anything, mock.Anything).
		Return(
			api.MinerInfo{
				PeerId: nil,
			}, nil,
		)
	queryAsk := getModule(t, mockGateway)
	assert.NotNil(queryAsk)
	ctx := context.Background()
	result, err := queryAsk.QueryMiner(ctx, "f01000")
	assert.Nil(err)
	assert.Equal(result.Status, NoPeerID)
	assert.Equal(result.ErrorMessage, "")
}

func TestQueryAsk_QueryMiner_InvalidMultiAddress(t *testing.T) {
	assert := assert.New(t)
	mockGateway := new(module.MockGateway)
	mockGateway.On("StateMinerInfo", mock.Anything, mock.Anything, mock.Anything).
		Return(
			api.MinerInfo{
				PeerId: new(peer.ID),
				Multiaddrs: []abi.Multiaddrs{
					[]byte("aaa"),
				},
			}, nil,
		)
	queryAsk := getModule(t, mockGateway)
	assert.NotNil(queryAsk)
	ctx := context.Background()
	result, err := queryAsk.QueryMiner(ctx, "f01000")
	assert.Nil(err)
	assert.Equal(result.Status, InvalidMultiAddress)
	assert.Equal(result.ErrorMessage, "no protocol with code 97")
}

func TestQueryAsk_QueryMiner_NoMultiAddr(t *testing.T) {
	assert := assert.New(t)
	queryAsk := getModule(t, nil)
	assert.NotNil(queryAsk)
	ctx := context.Background()
	result, err := queryAsk.QueryMiner(ctx, "f0688165")
	assert.Nil(err)
	assert.Equal(NoMultiAddress, result.Status)
	assert.Equal("miner has no multi address", result.ErrorMessage)
}

func TestQueryAsk_QueryMiner_CannotConnect(t *testing.T) {
	assert := assert.New(t)
	queryAsk := getModule(t, nil)
	assert.NotNil(queryAsk)
	ctx := context.Background()
	result, err := queryAsk.QueryMiner(ctx, "f01049918")
	assert.Nil(err)
	assert.Equal(result.Status, CannotConnect)
	assert.Contains(result.ErrorMessage, "failed to dial")
}

func TestQueryAsk_QueryMiner_Success(t *testing.T) {
	assert := assert.New(t)
	queryAsk := getModule(t, nil)
	assert.NotNil(queryAsk)
	ctx := context.Background()
	result, err := queryAsk.QueryMiner(ctx, "f01873432")
	assert.Nil(err)
	assert.Equal(result.Status, Success)
	assert.Greater(result.Latency.Seconds(), float64(0))
	assert.Equal(result.ErrorMessage, "")
	assert.Equal(uint64(0x800000000), result.MaxPieceSize)
}

func TestQueryAsk_Validate_Failed(t *testing.T) {
	assert := assert.New(t)
	queryAsk := getModule(t, nil)
	assert.NotNil(queryAsk)
	ctx := context.Background()
	result, err := queryAsk.Validate(
		ctx, module.ValidationInput{
			Task: task.Task{
				Target: "f01000",
			},
			Input: pgtype.JSONB{},
		},
	)
	assert.Nil(err)
	assert.Equal("f01000", result.Task.Target)
	assert.Equal(
		`{"status":"no_multi_address","errorMessage":"miner has no multi address"}`,
		string(result.Result.Bytes),
	)
}

func TestQueryAsk_Validate_Success(t *testing.T) {
	assert := assert.New(t)
	queryAsk := getModule(t, nil)
	assert.NotNil(queryAsk)
	ctx := context.Background()
	result, err := queryAsk.Validate(
		ctx, module.ValidationInput{
			Task: task.Task{
				Target: "f01873432",
			},
			Input: pgtype.JSONB{},
		},
	)
	assert.Nil(err)
	assert.Equal("f01873432", result.Task.Target)
	assert.Contains(string(result.Result.Bytes), `"status":"success"`)
}
