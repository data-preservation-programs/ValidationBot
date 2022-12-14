package mock

import (
	"context"
	"time"
	"validation-bot/module/retrieval"

	"github.com/filecoin-project/go-address"
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
) (*retrieval.ResultContent, error) {
	args := m.Called(parent, minerAddress, dataCid, timeout)
	return args.Get(0).(*retrieval.ResultContent), args.Error(1)
}

type MockGraphSyncRetrieverBuilder struct {
	Retriever *MockGraphSyncRetriever
}

func (m *MockGraphSyncRetrieverBuilder) Build() (retrieval.GraphSyncRetriever, retrieval.Cleanup, error) {
	return m.Retriever, func() {}, nil
}
