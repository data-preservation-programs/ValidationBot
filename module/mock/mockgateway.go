package mock

import (
	"context"

	"github.com/filecoin-project/go-address"
	"github.com/filecoin-project/go-jsonrpc"
	"github.com/filecoin-project/go-state-types/abi"
	"github.com/filecoin-project/go-state-types/builtin/v9/miner"
	"github.com/filecoin-project/go-state-types/dline"
	"github.com/filecoin-project/go-state-types/network"
	"github.com/filecoin-project/lotus/api"
	apitypes "github.com/filecoin-project/lotus/api/types"
	"github.com/filecoin-project/lotus/chain/types"
	"github.com/filecoin-project/lotus/chain/types/ethtypes"
	"github.com/filecoin-project/lotus/node/modules/dtypes"
	"github.com/ipfs/go-cid"
	blocks "github.com/ipfs/go-libipfs/blocks"
	"github.com/stretchr/testify/mock"
)

// nolint:lll
type MockGateway struct {
	mock.Mock
}

func (m *MockGateway) ChainGetParentMessages(ctx context.Context, c cid.Cid) ([]api.Message, error) {
	panic("implement me")
}

func (m *MockGateway) ChainGetParentReceipts(ctx context.Context, c cid.Cid) ([]*types.MessageReceipt, error) {
	panic("implement me")
}

func (m *MockGateway) ChainGetPath(ctx context.Context, from, to types.TipSetKey) ([]*api.HeadChange, error) {
	panic("implement me")
}

func (m *MockGateway) ChainGetTipSetAfterHeight(
	ctx context.Context,
	h abi.ChainEpoch,
	tsk types.TipSetKey,
) (*types.TipSet, error) {
	panic("implement me")
}

func (m *MockGateway) ChainGetGenesis(ctx context.Context) (*types.TipSet, error) {
	panic("implement me")
}

func (m *MockGateway) MsigGetVestingSchedule(
	ctx context.Context,
	addr address.Address,
	tsk types.TipSetKey,
) (api.MsigVesting, error) {
	panic("implement me")
}

func (m *MockGateway) StateReadState(ctx context.Context, actor address.Address, tsk types.TipSetKey) (
	*api.ActorState,
	error,
) {
	panic("implement me")
}

func (m *MockGateway) StateSearchMsg(
	ctx context.Context,
	from types.TipSetKey,
	msg cid.Cid,
	limit abi.ChainEpoch,
	allowReplaced bool,
) (*api.MsgLookup, error) {
	panic("implement me")
}

func (m *MockGateway) StateWaitMsg(
	ctx context.Context,
	cid cid.Cid,
	confidence uint64,
	limit abi.ChainEpoch,
	allowReplaced bool,
) (*api.MsgLookup, error) {
	panic("implement me")
}

func (m *MockGateway) Discover(ctx context.Context) (apitypes.OpenRPCDocument, error) {
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
	panic("implement me")
}

func (m *MockGateway) ChainPutObj(ctx context.Context, block blocks.Block) error {
	panic("implement me")
}

func (m *MockGateway) ChainHead(ctx context.Context) (*types.TipSet, error) {
	panic("implement me")
}

func (m *MockGateway) ChainGetBlockMessages(ctx context.Context, cid cid.Cid) (*api.BlockMessages, error) {
	panic("implement me")
}

func (m *MockGateway) ChainGetMessage(ctx context.Context, mc cid.Cid) (*types.Message, error) {
	panic("implement me")
}

func (m *MockGateway) ChainGetTipSet(ctx context.Context, tsk types.TipSetKey) (*types.TipSet, error) {
	panic("implement me")
}

func (m *MockGateway) ChainGetTipSetByHeight(ctx context.Context, h abi.ChainEpoch, tsk types.TipSetKey) (
	*types.TipSet,
	error,
) {
	panic("implement me")
}

func (m *MockGateway) ChainNotify(ctx context.Context) (<-chan []*api.HeadChange, error) {
	panic("implement me")
}

func (m *MockGateway) ChainReadObj(ctx context.Context, cid cid.Cid) ([]byte, error) {
	panic("implement me")
}

func (m *MockGateway) GasEstimateMessageGas(
	ctx context.Context,
	msg *types.Message,
	spec *api.MessageSendSpec,
	tsk types.TipSetKey,
) (*types.Message, error) {
	panic("implement me")
}

func (m *MockGateway) MpoolPush(ctx context.Context, sm *types.SignedMessage) (cid.Cid, error) {
	panic("implement me")
}

func (m *MockGateway) MsigGetAvailableBalance(
	ctx context.Context,
	addr address.Address,
	tsk types.TipSetKey,
) (types.BigInt, error) {
	panic("implement me")
}

func (m *MockGateway) MsigGetVested(
	ctx context.Context,
	addr address.Address,
	start types.TipSetKey,
	end types.TipSetKey,
) (types.BigInt, error) {
	panic("implement me")
}

func (m *MockGateway) MsigGetPending(
	ctx context.Context,
	address address.Address,
	key types.TipSetKey,
) ([]*api.MsigTransaction, error) {
	panic("implement me")
}

func (m *MockGateway) StateAccountKey(ctx context.Context, addr address.Address, tsk types.TipSetKey) (
	address.Address,
	error,
) {
	panic("implement me")
}

func (m *MockGateway) StateDealProviderCollateralBounds(
	ctx context.Context,
	size abi.PaddedPieceSize,
	verified bool,
	tsk types.TipSetKey,
) (api.DealCollateralBounds, error) {
	panic("implement me")
}

func (m *MockGateway) StateGetActor(ctx context.Context, actor address.Address, ts types.TipSetKey) (
	*types.Actor,
	error,
) {
	panic("implement me")
}

func (m *MockGateway) StateGetReceipt(ctx context.Context, cid cid.Cid, key types.TipSetKey) (
	*types.MessageReceipt,
	error,
) {
	panic("implement me")
}

func (m *MockGateway) StateListMiners(ctx context.Context, tsk types.TipSetKey) ([]address.Address, error) {
	panic("implement me")
}

func (m *MockGateway) StateLookupID(ctx context.Context, addr address.Address, tsk types.TipSetKey) (
	address.Address,
	error,
) {
	panic("implement me")
}

func (m *MockGateway) StateMarketBalance(
	ctx context.Context,
	addr address.Address,
	tsk types.TipSetKey,
) (api.MarketBalance, error) {
	panic("implement me")
}

func (m *MockGateway) StateMarketStorageDeal(
	ctx context.Context,
	dealID abi.DealID,
	tsk types.TipSetKey,
) (*api.MarketDeal, error) {
	panic("implement me")
}

func (m *MockGateway) StateMinerProvingDeadline(
	ctx context.Context,
	addr address.Address,
	tsk types.TipSetKey,
) (*dline.Info, error) {
	panic("implement me")
}

func (m *MockGateway) StateMinerPower(
	ctx context.Context,
	address address.Address,
	key types.TipSetKey,
) (*api.MinerPower, error) {
	panic("implement me")
}

func (m *MockGateway) StateNetworkVersion(ctx context.Context, key types.TipSetKey) (network.Version, error) {
	panic("implement me")
}

func (m *MockGateway) StateSectorGetInfo(
	ctx context.Context,
	maddr address.Address,
	n abi.SectorNumber,
	tsk types.TipSetKey,
) (*miner.SectorOnChainInfo, error) {
	panic("implement me")
}

func (m *MockGateway) StateVerifiedClientStatus(
	ctx context.Context,
	addr address.Address,
	tsk types.TipSetKey,
) (*abi.StoragePower, error) {
	panic("implement me")
}

func (m *MockGateway) WalletBalance(ctx context.Context, address address.Address) (types.BigInt, error) {
	panic("implement me")
}

func (m *MockGateway) Version(ctx context.Context) (api.APIVersion, error) {
	panic("implement me")
}

func (m *MockGateway) EthAccounts(ctx context.Context) ([]ethtypes.EthAddress, error) {
	panic("implement me")
}

func (m *MockGateway) EthBlockNumber(ctx context.Context) (ethtypes.EthUint64, error) {
	panic("implement me")
}

func (m *MockGateway) EthGetBlockTransactionCountByNumber(ctx context.Context, blkNum ethtypes.EthUint64) (
	ethtypes.EthUint64,
	error,
) {
	panic("implement me")
}

func (m *MockGateway) EthGetBlockTransactionCountByHash(ctx context.Context, blkHash ethtypes.EthHash) (
	ethtypes.EthUint64,
	error,
) {
	panic("implement me")
}

func (m *MockGateway) EthGetBlockByHash(ctx context.Context, blkHash ethtypes.EthHash, fullTxInfo bool) (
	ethtypes.EthBlock,
	error,
) {
	panic("implement me")
}

func (m *MockGateway) EthGetBlockByNumber(ctx context.Context, blkNum string, fullTxInfo bool) (
	ethtypes.EthBlock,
	error,
) {
	panic("implement me")
}

func (m *MockGateway) EthGetTransactionByHash(ctx context.Context, txHash *ethtypes.EthHash) (
	*ethtypes.EthTx,
	error,
) {
	panic("implement me")
}

func (m *MockGateway) EthGetTransactionByHashLimited(
	ctx context.Context,
	txHash *ethtypes.EthHash,
	limit abi.ChainEpoch,
) (
	*ethtypes.EthTx,
	error,
) {
	panic("implement me")
}

func (m *MockGateway) EthGetTransactionHashByCid(ctx context.Context, cid cid.Cid) (*ethtypes.EthHash, error) {
	panic("implement me")
}

func (m *MockGateway) EthGetMessageCidByTransactionHash(ctx context.Context, txHash *ethtypes.EthHash) (
	*cid.Cid,
	error,
) {
	panic("implement me")
}

func (m *MockGateway) EthGetTransactionCount(ctx context.Context, sender ethtypes.EthAddress, blkOpt string) (
	ethtypes.EthUint64,
	error,
) {
	panic("implement me")
}

func (m *MockGateway) EthGetTransactionReceipt(ctx context.Context, txHash ethtypes.EthHash) (
	*api.EthTxReceipt,
	error,
) {
	panic("implement me")
}

func (m *MockGateway) EthGetTransactionReceiptLimited(
	ctx context.Context,
	txHash ethtypes.EthHash,
	limit abi.ChainEpoch,
) (
	*api.EthTxReceipt,
	error,
) {
	panic("implement me")
}

func (m *MockGateway) EthGetCode(ctx context.Context, address ethtypes.EthAddress, blkOpt string) (
	ethtypes.EthBytes,
	error,
) {
	panic("implement me")
}

func (m *MockGateway) EthGetStorageAt(
	ctx context.Context,
	address ethtypes.EthAddress,
	position ethtypes.EthBytes,
	blkParam string,
) (
	ethtypes.EthBytes,
	error,
) {
	panic("implement me")
}

func (m *MockGateway) EthGetBalance(ctx context.Context, address ethtypes.EthAddress, blkParam string) (
	ethtypes.EthBigInt,
	error,
) {
	panic("implement me")
}

// nolint:stylecheck
func (m *MockGateway) EthChainId(ctx context.Context) (ethtypes.EthUint64, error) {
	panic("implement me")
}

func (m *MockGateway) NetVersion(ctx context.Context) (string, error) {
	panic("implement me")
}

func (m *MockGateway) NetListening(ctx context.Context) (bool, error) {
	panic("implement me")
}

func (m *MockGateway) EthProtocolVersion(ctx context.Context) (ethtypes.EthUint64, error) {
	panic("implement me")
}

func (m *MockGateway) EthGasPrice(ctx context.Context) (ethtypes.EthBigInt, error) {
	panic("implement me")
}

func (m *MockGateway) EthFeeHistory(ctx context.Context, p jsonrpc.RawParams) (ethtypes.EthFeeHistory, error) {
	panic("implement me")
}

func (m *MockGateway) EthMaxPriorityFeePerGas(ctx context.Context) (ethtypes.EthBigInt, error) {
	panic("implement me")
}

func (m *MockGateway) EthEstimateGas(ctx context.Context, tx ethtypes.EthCall) (ethtypes.EthUint64, error) {
	panic("implement me")
}

func (m *MockGateway) EthCall(ctx context.Context, tx ethtypes.EthCall, blkParam string) (
	ethtypes.EthBytes,
	error,
) {
	panic("implement me")
}

func (m *MockGateway) EthSendRawTransaction(ctx context.Context, rawTx ethtypes.EthBytes) (
	ethtypes.EthHash,
	error,
) {
	panic("implement me")
}

func (m *MockGateway) EthGetLogs(ctx context.Context, filter *ethtypes.EthFilterSpec) (
	*ethtypes.EthFilterResult,
	error,
) {
	panic("implement me")
}

func (m *MockGateway) EthGetFilterChanges(ctx context.Context, id ethtypes.EthFilterID) (
	*ethtypes.EthFilterResult,
	error,
) {
	panic("implement me")
}

func (m *MockGateway) EthGetFilterLogs(ctx context.Context, id ethtypes.EthFilterID) (
	*ethtypes.EthFilterResult,
	error,
) {
	panic("implement me")
}

func (m *MockGateway) EthNewFilter(ctx context.Context, filter *ethtypes.EthFilterSpec) (
	ethtypes.EthFilterID,
	error,
) {
	panic("implement me")
}

func (m *MockGateway) EthNewBlockFilter(ctx context.Context) (ethtypes.EthFilterID, error) {
	panic("implement me")
}

func (m *MockGateway) EthNewPendingTransactionFilter(ctx context.Context) (ethtypes.EthFilterID, error) {
	panic("implement me")
}

func (m *MockGateway) EthUninstallFilter(ctx context.Context, id ethtypes.EthFilterID) (bool, error) {
	panic("implement me")
}

func (m *MockGateway) EthSubscribe(ctx context.Context, params jsonrpc.RawParams) (
	ethtypes.EthSubscriptionID,
	error,
) {
	panic("implement me")
}

func (m *MockGateway) EthUnsubscribe(ctx context.Context, id ethtypes.EthSubscriptionID) (bool, error) {
	panic("implement me")
}

func (m *MockGateway) Web3ClientVersion(ctx context.Context) (string, error) {
	panic("implement me")
}

// placeholder

func (m *MockGateway) MpoolGetNonce(ctx context.Context, addr address.Address) (uint64, error) {
	panic("implement me")
}

func (m *MockGateway) StateCall(ctx context.Context, msg *types.Message, tsk types.TipSetKey) (
	*api.InvocResult,
	error,
) {
	panic("implement me")
}

func (m *MockGateway) StateDecodeParams(
	ctx context.Context,
	toAddr address.Address,
	method abi.MethodNum,
	params []byte,
	tsk types.TipSetKey,
) (interface{}, error) {
	panic("implement me")
}

func (m *MockGateway) StateNetworkName(context.Context) (dtypes.NetworkName, error) {
	panic("implement me")
}

func (m *MockGateway) StateVerifierStatus(
	ctx context.Context,
	addr address.Address,
	tsk types.TipSetKey,
) (*abi.StoragePower, error) {
	panic("implement me")
}
