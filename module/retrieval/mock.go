package retrieval

import (
	"context"
	"time"

	"github.com/filecoin-project/go-address"
	blocks "github.com/ipfs/go-block-format"
	"github.com/ipfs/go-cid"
	bsmsg "github.com/ipfs/go-libipfs/bitswap/message"
	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/stretchr/testify/mock"
)

type MockGraphSyncRetriever struct {
	mock.Mock
}

//nolint:forcetypeassert
func (m *MockGraphSyncRetriever) Retrieve(
	parent context.Context,
	minerAddress address.Address,
	dataCid cid.Cid,
	timeout time.Duration,
) (*ResultContent, error) {
	args := m.Called(parent, minerAddress, dataCid, timeout)
	return args.Get(0).(*ResultContent), args.Error(1)
}

type MockGraphSyncRetrieverBuilder struct {
	Retriever *MockGraphSyncRetriever
}

func (m *MockGraphSyncRetrieverBuilder) Build() (GraphSyncRetriever, Cleanup, error) {
	return m.Retriever, func() {}, nil
}

type mockReadStore struct {
	mock.Mock
}

//nolint:forcetypeassert
func (m *mockReadStore) GetBlock(ctx context.Context, c cid.Cid) (blocks.Block, error) {
	args := m.Called(ctx, c)

	return args.Get(0).(blocks.Block), args.Error(1)
}

func (m *mockReadStore) Close() error {
	args := m.Called()

	return args.Error(0)
}

func (m *mockReadStore) ReceiveMessage(
	ctx context.Context,
	sender peer.ID,
	incoming bsmsg.BitSwapMessage) {
	m.Called()
}

func (m *mockReadStore) ReceiveError(error) {
	m.Called()
}

// Connected/Disconnected warns bitswap about peer connections.
func (m *mockReadStore) PeerConnected(peer.ID) {
	m.Called()
}
func (m *mockReadStore) PeerDisconnected(peer.ID) {
	m.Called()
}

type BitswapRetrieverMock struct {
	mock.Mock
}

//nolint:forcetypeassert,lll
func (b *BitswapRetrieverMock) Retrieve(ctx context.Context, root cid.Cid, timeout time.Duration) (
	*ResultContent,
	error,
) {
	args := b.Called(ctx, root, timeout)
	return args.Get(0).(*ResultContent), args.Error(1)
}
