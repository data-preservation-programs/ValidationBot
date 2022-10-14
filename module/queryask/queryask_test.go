package queryask

import (
	"context"
	"testing"
	"validation-bot/role"

	"github.com/filecoin-project/go-address"
	"github.com/filecoin-project/go-state-types/abi"
	"github.com/filecoin-project/go-state-types/builtin/v9/miner"
	"github.com/filecoin-project/go-state-types/dline"
	"github.com/filecoin-project/go-state-types/network"
	"github.com/filecoin-project/lotus/api"
	"github.com/filecoin-project/lotus/api/client"
	"github.com/filecoin-project/lotus/chain/types"
	"github.com/ipfs/go-block-format"
	"github.com/ipfs/go-cid"
	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

func getModule(t *testing.T, mockGateway *MockGateway) QueryAsk {
	assert := assert.New(t)
	priv, _, _, err := role.GenerateNewPeer()
	assert.Nil(err)
	libp2p, err := role.NewLibp2pHost(priv, "/ip4/0.0.0.0/tcp/0")
	assert.Nil(err)
	assert.NotNil(libp2p)
	ctx := context.Background()
	var queryAsk QueryAsk
	if mockGateway == nil {
		lotusApi, closer, err := client.NewGatewayRPCV0(ctx, "https://api.node.glif.io/", nil)
		defer closer()
		assert.Nil(err)
		queryAsk = NewQueryAskModule(libp2p, lotusApi)
	} else {
		queryAsk = NewQueryAskModule(libp2p, mockGateway)
	}
	return queryAsk
}

type MockGateway struct {
	mock.Mock
}

func (m *MockGateway) StateMinerInfo(ctx context.Context, actor address.Address, tsk types.TipSetKey) (api.MinerInfo, error) {
	args := m.Called(ctx, actor, tsk)
	return args.Get(0).(api.MinerInfo), args.Error(1)
}

func (m *MockGateway) ChainHasObj(ctx context.Context, cid cid.Cid) (bool, error) {
	//TODO implement me
	panic("implement me")
}

func (m *MockGateway) ChainPutObj(ctx context.Context, block blocks.Block) error {
	//TODO implement me
	panic("implement me")
}

func (m *MockGateway) ChainHead(ctx context.Context) (*types.TipSet, error) {
	//TODO implement me
	panic("implement me")
}

func (m *MockGateway) ChainGetBlockMessages(ctx context.Context, cid cid.Cid) (*api.BlockMessages, error) {
	//TODO implement me
	panic("implement me")
}

func (m *MockGateway) ChainGetMessage(ctx context.Context, mc cid.Cid) (*types.Message, error) {
	//TODO implement me
	panic("implement me")
}

func (m *MockGateway) ChainGetTipSet(ctx context.Context, tsk types.TipSetKey) (*types.TipSet, error) {
	//TODO implement me
	panic("implement me")
}

func (m *MockGateway) ChainGetTipSetByHeight(ctx context.Context, h abi.ChainEpoch, tsk types.TipSetKey) (*types.TipSet, error) {
	//TODO implement me
	panic("implement me")
}

func (m *MockGateway) ChainNotify(ctx context.Context) (<-chan []*api.HeadChange, error) {
	//TODO implement me
	panic("implement me")
}

func (m *MockGateway) ChainReadObj(ctx context.Context, cid cid.Cid) ([]byte, error) {
	//TODO implement me
	panic("implement me")
}

func (m *MockGateway) GasEstimateMessageGas(ctx context.Context, msg *types.Message, spec *api.MessageSendSpec, tsk types.TipSetKey) (*types.Message, error) {
	//TODO implement me
	panic("implement me")
}

func (m *MockGateway) MpoolPush(ctx context.Context, sm *types.SignedMessage) (cid.Cid, error) {
	//TODO implement me
	panic("implement me")
}

func (m *MockGateway) MsigGetAvailableBalance(ctx context.Context, addr address.Address, tsk types.TipSetKey) (types.BigInt, error) {
	//TODO implement me
	panic("implement me")
}

func (m *MockGateway) MsigGetVested(ctx context.Context, addr address.Address, start types.TipSetKey, end types.TipSetKey) (types.BigInt, error) {
	//TODO implement me
	panic("implement me")
}

func (m *MockGateway) MsigGetPending(ctx context.Context, address address.Address, key types.TipSetKey) ([]*api.MsigTransaction, error) {
	//TODO implement me
	panic("implement me")
}

func (m *MockGateway) StateAccountKey(ctx context.Context, addr address.Address, tsk types.TipSetKey) (address.Address, error) {
	//TODO implement me
	panic("implement me")
}

func (m *MockGateway) StateDealProviderCollateralBounds(ctx context.Context, size abi.PaddedPieceSize, verified bool, tsk types.TipSetKey) (api.DealCollateralBounds, error) {
	//TODO implement me
	panic("implement me")
}

func (m *MockGateway) StateGetActor(ctx context.Context, actor address.Address, ts types.TipSetKey) (*types.Actor, error) {
	//TODO implement me
	panic("implement me")
}

func (m *MockGateway) StateGetReceipt(ctx context.Context, cid cid.Cid, key types.TipSetKey) (*types.MessageReceipt, error) {
	//TODO implement me
	panic("implement me")
}

func (m *MockGateway) StateListMiners(ctx context.Context, tsk types.TipSetKey) ([]address.Address, error) {
	//TODO implement me
	panic("implement me")
}

func (m *MockGateway) StateLookupID(ctx context.Context, addr address.Address, tsk types.TipSetKey) (address.Address, error) {
	//TODO implement me
	panic("implement me")
}

func (m *MockGateway) StateMarketBalance(ctx context.Context, addr address.Address, tsk types.TipSetKey) (api.MarketBalance, error) {
	//TODO implement me
	panic("implement me")
}

func (m *MockGateway) StateMarketStorageDeal(ctx context.Context, dealId abi.DealID, tsk types.TipSetKey) (*api.MarketDeal, error) {
	//TODO implement me
	panic("implement me")
}

func (m *MockGateway) StateMinerProvingDeadline(ctx context.Context, addr address.Address, tsk types.TipSetKey) (*dline.Info, error) {
	//TODO implement me
	panic("implement me")
}

func (m *MockGateway) StateMinerPower(ctx context.Context, address address.Address, key types.TipSetKey) (*api.MinerPower, error) {
	//TODO implement me
	panic("implement me")
}

func (m *MockGateway) StateNetworkVersion(ctx context.Context, key types.TipSetKey) (network.Version, error) {
	//TODO implement me
	panic("implement me")
}

func (m *MockGateway) StateSearchMsg(ctx context.Context, msg cid.Cid) (*api.MsgLookup, error) {
	//TODO implement me
	panic("implement me")
}

func (m *MockGateway) StateSectorGetInfo(ctx context.Context, maddr address.Address, n abi.SectorNumber, tsk types.TipSetKey) (*miner.SectorOnChainInfo, error) {
	//TODO implement me
	panic("implement me")
}

func (m *MockGateway) StateVerifiedClientStatus(ctx context.Context, addr address.Address, tsk types.TipSetKey) (*abi.StoragePower, error) {
	//TODO implement me
	panic("implement me")
}

func (m *MockGateway) StateWaitMsg(ctx context.Context, msg cid.Cid, confidence uint64) (*api.MsgLookup, error) {
	//TODO implement me
	panic("implement me")
}

func (m *MockGateway) WalletBalance(ctx context.Context, address address.Address) (types.BigInt, error) {
	//TODO implement me
	panic("implement me")
}

func (m *MockGateway) Version(ctx context.Context) (api.APIVersion, error) {
	//TODO implement me
	panic("implement me")
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
	mockGateway := new(MockGateway)
	mockGateway.On("StateMinerInfo", mock.Anything, mock.Anything, mock.Anything).
		Return(api.MinerInfo{
			PeerId: nil,
		}, nil)
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
	mockGateway := new(MockGateway)
	mockGateway.On("StateMinerInfo", mock.Anything, mock.Anything, mock.Anything).
		Return(api.MinerInfo{
			PeerId: new(peer.ID),
			Multiaddrs: []abi.Multiaddrs{
				[]byte("aaa"),
			},
		}, nil)
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
	assert.Equal(result.Status, NoMultiAddress)
	assert.Equal(result.ErrorMessage, "")
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
	assert.Equal(result.ErrorMessage, "")
	assert.Equal(uint64(0x800000000), result.MaxPieceSize)
}

func TestQueryAsk_Validate_Failed(t *testing.T) {
	assert := assert.New(t)
	queryAsk := getModule(t, nil)
	assert.NotNil(queryAsk)
	ctx := context.Background()
	result, err := queryAsk.Validate(ctx, []byte(`{"target":"f01000"}`))
	assert.Nil(err)
	assert.Equal(`{"status":"no_multi_address"}`, string(result))
}

func TestQueryAsk_Validate_Success(t *testing.T) {
	assert := assert.New(t)
	queryAsk := getModule(t, nil)
	assert.NotNil(queryAsk)
	ctx := context.Background()
	result, err := queryAsk.Validate(ctx, []byte(`{"target":"f01873432"}`))
	assert.Nil(err)
	assert.Equal(`{"peerId":"12D3KooWDWtfzYYeThH5WjXRurf723BeqP9EJ55mKSXhSFx899Pk","multiAddrs":["/ip4/38.70.220.54/tcp/10201"],"status":"success","price":"20000000000","verifiedPrice":"0","minPieceSize":8589934592,"maxPieceSize":34359738368}`, string(result))
}
