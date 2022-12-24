package mockResultPublisher

import (
	"context"
	"io"
	"time"
	"validation-bot/module"

	"github.com/stretchr/testify/mock"
)

type MockRPCClient struct {
	mock.Mock

	Timeout time.Duration
}

func (m *MockRPCClient) CallServer(ctx context.Context, input module.ValidationInput) (*module.ValidationResult, error) {
	args := m.Called(ctx, input)

	value, ok := args.Get(0).(*module.ValidationResult)

	if !ok {
		return nil, args.Error(1)
	}

	return value, args.Error(1)
}

func (m *MockRPCClient) GetTimeout() time.Duration {
	return m.Timeout
}

func (m *MockRPCClient) Validate(ctx context.Context, stdout io.Reader, input module.ValidationInput) (*module.ValidationResult, error) {
	args := m.Called(ctx, stdout, input)

	value, ok := args.Get(0).(*module.ValidationResult)

	if !ok {
		return nil, args.Error(1)
	}

	return value, args.Error(1)
}
