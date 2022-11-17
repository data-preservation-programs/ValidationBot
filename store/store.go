package store

import (
	"context"

	"github.com/ipfs/go-cid"
	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/stretchr/testify/mock"
)

type Publisher interface {
	Publish(ctx context.Context, input []byte) error
}

type Entry struct {
	Message  []byte
	Previous *cid.Cid
	CID      cid.Cid
}

type Subscriber interface {
	Subscribe(ctx context.Context, peerID peer.ID, last *cid.Cid, oneOff bool) (<-chan Entry, error)
}

type Store interface {
	Publisher
	Subscriber
}

type MockSubscriber struct {
	mock.Mock
}

//nolint:all
func (m *MockSubscriber) Subscribe(ctx context.Context, peerID peer.ID, last *cid.Cid, oneOff bool) (
	<-chan Entry,
	error,
) {
	args := m.Called(ctx, peerID, last, oneOff)
	return args.Get(0).(<-chan Entry), args.Error(1)
}

type MockPublisher struct {
	mock.Mock
}

func (m *MockPublisher) Publish(ctx context.Context, input []byte) error {
	args := m.Called(ctx, input)
	return args.Error(0)
}
