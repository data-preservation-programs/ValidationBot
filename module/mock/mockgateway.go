package mock

import (
	"context"

	"github.com/filecoin-project/go-address"
	"github.com/filecoin-project/go-state-types/abi"
	"github.com/filecoin-project/go-state-types/builtin/v9/miner"
	"github.com/filecoin-project/go-state-types/dline"
	"github.com/filecoin-project/go-state-types/network"
	"github.com/filecoin-project/lotus/api"
	apitypes "github.com/filecoin-project/lotus/api/types"
	"github.com/filecoin-project/lotus/chain/types"
	"github.com/ipfs/go-cid"
	blocks "github.com/ipfs/go-libipfs/blocks"
	"github.com/stretchr/testify/mock"
)

type MockGateway struct {
	mock.Mock
}

func (m *MockGateway) ChainGetParentMessages(ctx context.Context, c cid.Cid) ([]api.Message, error) {
	// TODO implement me
	panic("implement me")
}

func (m *MockGateway) ChainGetParentReceipts(ctx context.Context, c cid.Cid) ([]*types.MessageReceipt, error) {
	// TODO implement me
	panic("implement me")
}

func (m *MockGateway) ChainGetPath(ctx context.Context, from, to types.TipSetKey) ([]*api.HeadChange, error) {
	// TODO implement me
	panic("implement me")
}

func (m *MockGateway) ChainGetTipSetAfterHeight(
	ctx context.Context,
	h abi.ChainEpoch,
	tsk types.TipSetKey,
) (*types.TipSet, error) {
	// TODO implement me
	panic("implement me")
}

func (m *MockGateway) ChainGetGenesis(ctx context.Context) (*types.TipSet, error) {
	// TODO implement me
	panic("implement me")
}

func (m *MockGateway) MsigGetVestingSchedule(
	ctx context.Context,
	addr address.Address,
	tsk types.TipSetKey,
) (api.MsigVesting, error) {
	// TODO implement me
	panic("implement me")
}

func (m *MockGateway) StateReadState(ctx context.Context, actor address.Address, tsk types.TipSetKey) (
	*api.ActorState,
	error,
) {
	// TODO implement me
	panic("implement me")
}

func (m *MockGateway) StateSearchMsg(
	ctx context.Context,
	from types.TipSetKey,
	msg cid.Cid,
	limit abi.ChainEpoch,
	allowReplaced bool,
) (*api.MsgLookup, error) {
	// TODO implement me
	panic("implement me")
}

func (m *MockGateway) StateWaitMsg(
	ctx context.Context,
	cid cid.Cid,
	confidence uint64,
	limit abi.ChainEpoch,
	allowReplaced bool,
) (*api.MsgLookup, error) {
	// TODO implement me
	panic("implement me")
}

func (m *MockGateway) Discover(ctx context.Context) (apitypes.OpenRPCDocument, error) {
	// TODO implement me
	panic("implement me")
}

//nolint:forcetypeassert
func (m *MockGateway) StateMinerInfo(ctx context.Context, actor address.Address, tsk types.TipSetKey) (
	api.MinerInfo,
	error,
) {
	args := m.Called(ctx, actor, tsk)
	return args.Get(0).(api.MinerInfo), args.Error(1)
}

func (m *MockGateway) ChainHasObj(ctx context.Context, cid cid.Cid) (bool, error) {
	// TODO implement me
	panic("implement me")
}

func (m *MockGateway) ChainPutObj(ctx context.Context, block blocks.Block) error {
	// TODO implement me
	panic("implement me")
}

func (m *MockGateway) ChainHead(ctx context.Context) (*types.TipSet, error) {
	// TODO implement me
	panic("implement me")
}

func (m *MockGateway) ChainGetBlockMessages(ctx context.Context, cid cid.Cid) (*api.BlockMessages, error) {
	// TODO implement me
	panic("implement me")
}

func (m *MockGateway) ChainGetMessage(ctx context.Context, mc cid.Cid) (*types.Message, error) {
	// TODO implement me
	panic("implement me")
}

func (m *MockGateway) ChainGetTipSet(ctx context.Context, tsk types.TipSetKey) (*types.TipSet, error) {
	// TODO implement me
	panic("implement me")
}

func (m *MockGateway) ChainGetTipSetByHeight(ctx context.Context, h abi.ChainEpoch, tsk types.TipSetKey) (
	*types.TipSet,
	error,
) {
	// TODO implement me
	panic("implement me")
}

func (m *MockGateway) ChainNotify(ctx context.Context) (<-chan []*api.HeadChange, error) {
	// TODO implement me
	panic("implement me")
}

func (m *MockGateway) ChainReadObj(ctx context.Context, cid cid.Cid) ([]byte, error) {
	// TODO implement me
	panic("implement me")
}

func (m *MockGateway) GasEstimateMessageGas(
	ctx context.Context,
	msg *types.Message,
	spec *api.MessageSendSpec,
	tsk types.TipSetKey,
) (*types.Message, error) {
	// TODO implement me
	panic("implement me")
}

func (m *MockGateway) MpoolPush(ctx context.Context, sm *types.SignedMessage) (cid.Cid, error) {
	// TODO implement me
	panic("implement me")
}

func (m *MockGateway) MsigGetAvailableBalance(
	ctx context.Context,
	addr address.Address,
	tsk types.TipSetKey,
) (types.BigInt, error) {
	// TODO implement me
	panic("implement me")
}

func (m *MockGateway) MsigGetVested(
	ctx context.Context,
	addr address.Address,
	start types.TipSetKey,
	end types.TipSetKey,
) (types.BigInt, error) {
	// TODO implement me
	panic("implement me")
}

func (m *MockGateway) MsigGetPending(
	ctx context.Context,
	address address.Address,
	key types.TipSetKey,
) ([]*api.MsigTransaction, error) {
	// TODO implement me
	panic("implement me")
}

func (m *MockGateway) StateAccountKey(ctx context.Context, addr address.Address, tsk types.TipSetKey) (
	address.Address,
	error,
) {
	// TODO implement me
	panic("implement me")
}

func (m *MockGateway) StateDealProviderCollateralBounds(
	ctx context.Context,
	size abi.PaddedPieceSize,
	verified bool,
	tsk types.TipSetKey,
) (api.DealCollateralBounds, error) {
	// TODO implement me
	panic("implement me")
}

func (m *MockGateway) StateGetActor(ctx context.Context, actor address.Address, ts types.TipSetKey) (
	*types.Actor,
	error,
) {
	// TODO implement me
	panic("implement me")
}

func (m *MockGateway) StateGetReceipt(ctx context.Context, cid cid.Cid, key types.TipSetKey) (
	*types.MessageReceipt,
	error,
) {
	// TODO implement me
	panic("implement me")
}

func (m *MockGateway) StateListMiners(ctx context.Context, tsk types.TipSetKey) ([]address.Address, error) {
	// TODO implement me
	panic("implement me")
}

func (m *MockGateway) StateLookupID(ctx context.Context, addr address.Address, tsk types.TipSetKey) (
	address.Address,
	error,
) {
	// TODO implement me
	panic("implement me")
}

func (m *MockGateway) StateMarketBalance(
	ctx context.Context,
	addr address.Address,
	tsk types.TipSetKey,
) (api.MarketBalance, error) {
	// TODO implement me
	panic("implement me")
}

func (m *MockGateway) StateMarketStorageDeal(
	ctx context.Context,
	dealID abi.DealID,
	tsk types.TipSetKey,
) (*api.MarketDeal, error) {
	// TODO implement me
	panic("implement me")
}

func (m *MockGateway) StateMinerProvingDeadline(
	ctx context.Context,
	addr address.Address,
	tsk types.TipSetKey,
) (*dline.Info, error) {
	// TODO implement me
	panic("implement me")
}

func (m *MockGateway) StateMinerPower(
	ctx context.Context,
	address address.Address,
	key types.TipSetKey,
) (*api.MinerPower, error) {
	// TODO implement me
	panic("implement me")
}

func (m *MockGateway) StateNetworkVersion(ctx context.Context, key types.TipSetKey) (network.Version, error) {
	// TODO implement me
	panic("implement me")
}

func (m *MockGateway) StateSectorGetInfo(
	ctx context.Context,
	maddr address.Address,
	n abi.SectorNumber,
	tsk types.TipSetKey,
) (*miner.SectorOnChainInfo, error) {
	// TODO implement me
	panic("implement me")
}

func (m *MockGateway) StateVerifiedClientStatus(
	ctx context.Context,
	addr address.Address,
	tsk types.TipSetKey,
) (*abi.StoragePower, error) {
	// TODO implement me
	panic("implement me")
}

func (m *MockGateway) WalletBalance(ctx context.Context, address address.Address) (types.BigInt, error) {
	// TODO implement me
	panic("implement me")
}

func (m *MockGateway) Version(ctx context.Context) (api.APIVersion, error) {
	// TODO implement me
	panic("implement me")
}
