package retrieval

import (
	"context"
	"time"

	"github.com/filecoin-project/go-address"
	blocks "github.com/ipfs/go-block-format"
	"github.com/ipfs/go-cid"
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

func (m *mockReadStore) Get(ctx context.Context, c cid.Cid) (blocks.Block, error) {
	args := m.Called(ctx, c)

	return args.Get(0).(blocks.Block), args.Error(1)
}

type BitswapRetrieverMock struct {
	mock.Mock
}

func (b *BitswapRetrieverMock) Retrieve(ctx context.Context, root cid.Cid, timeout time.Duration) (*ResultContent, error) {
	args := b.Called(ctx, root, timeout)
	return args.Get(0).(*ResultContent), args.Error(1)
}
