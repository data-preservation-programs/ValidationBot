// nolint:stylecheck
package mockResultPublisher

import (
	"context"
	"time"
	"validation-bot/module"
	"validation-bot/task"

	"github.com/stretchr/testify/mock"
)

type MockRPCClient struct {
	mock.Mock

	Timeout time.Duration
}

func (m *MockRPCClient) CallServer(
	ctx context.Context,
	dir string,
	input module.ValidationInput,
) (*module.ValidationResult, error) {
	args := m.Called(ctx, dir, input)

	value, ok := args.Get(0).(*module.ValidationResult)

	if !ok {
		return nil, args.Error(1)
	}

	return value, args.Error(1)
}

func (m *MockRPCClient) GetTimeout() time.Duration {
	return m.Timeout
}

func (m *MockRPCClient) CreateTmpDir(modType task.Type) (string, error) {
	args := m.Called(modType)

	value, ok := args.Get(0).(string)

	if !ok {
		return "", args.Error(1)
	}

	return value, args.Error(1)
}

func (m *MockRPCClient) Validate(
	ctx context.Context,
	port int,
	input module.ValidationInput,
) (*module.ValidationResult, error) {
	args := m.Called(ctx, port, input)

	value, ok := args.Get(0).(*module.ValidationResult)

	if !ok {
		return nil, args.Error(1)
	}

	return value, args.Error(1)
}
