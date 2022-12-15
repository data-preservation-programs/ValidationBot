package mock

import (
	"context"
	"validation-bot/store"

	"github.com/ipfs/go-cid"
	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/stretchr/testify/mock"
)

type MockSubscriber struct {
	mock.Mock
}

//nolint:all
func (m *MockSubscriber) Subscribe(ctx context.Context, peerID peer.ID, last *cid.Cid, oneOff bool) (
	<-chan store.Entry,
	error,
) {
	args := m.Called(ctx, peerID, last, oneOff)
	return args.Get(0).(<-chan store.Entry), args.Error(1)
}

type MockPublisher struct {
	mock.Mock
}

func (m *MockPublisher) Publish(ctx context.Context, input store.MessagePayload) error {
	args := m.Called(ctx, input)
	return args.Error(0)
}
