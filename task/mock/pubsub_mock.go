package mock

import (
	"context"
	"validation-bot/task"

	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/stretchr/testify/mock"
)

type MockPublisherSubscriber struct {
	mock.Mock
}

func (m *MockPublisherSubscriber) Publish(ctx context.Context, task task.Payload) error {
	args := m.Called(ctx, task)
	return args.Error(0)
}

//nolint:all
func (m *MockPublisherSubscriber) Next(ctx context.Context) (*peer.ID, task.Payload, error) {
	args := m.Called(ctx)
	return args.Get(0).(*peer.ID), args.Get(1).([]byte), args.Error(2)
}
