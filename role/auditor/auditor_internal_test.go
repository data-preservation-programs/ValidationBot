package auditor

import (
	"context"
	"testing"
	"time"

	"validation-bot/task"

	"validation-bot/test"

	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

const (
	TestType task.Type = "module"
)

type MockValidator struct {
	mock.Mock
}

func (m *MockValidator) Validate(ctx context.Context, input []byte) (output []byte, err error) {
	args := m.Called(ctx, input)
	return args.Get(0).([]byte), args.Error(1)
}

type MockPublisher struct {
	mock.Mock
}

func (m *MockPublisher) Publish(ctx context.Context, input []byte) error {
	args := m.Called(ctx, input)
	return args.Error(0)
}

func TestAuditor_Start(t *testing.T) {
	assert := assert.New(t)
	_, _, peerID := test.GeneratePeerID(t)
	topic, pubPort, subPort := "module", 5556, 5557
	mockPublisher := new(MockPublisher)
	mockPublisher.On("Publish", mock.Anything, mock.Anything).Return(nil)

	auditor, err := NewAuditor(Config{
		TrustedPeers: []peer.ID{},
	})
	assert.Nil(err)

	mockValidator := new(MockValidator)
	mockValidator.On("Validate", mock.Anything, mock.Anything).Return([]byte("test_output"), nil)

	ctx, cancel := context.WithTimeout(context.Background(), time.Second*3)
	defer cancel()
	task := []byte("{\"type\":\"module\",\"target\":\"target\",\"input\":\"hello\"}")
	go func(ctx context.Context) {
		test.PublishTask(ctx, t, pubPort, subPort, peerID, topic, task)
		<-ctx.Done()
	}(ctx)
	auditor.Start(ctx)

	// Verify the module validator has been called with the module task
	mockValidator.AssertCalled(t, "Validate", mock.Anything, task)
	// Verify the message has been published
	mockPublisher.AssertCalled(t, "Publish", mock.Anything, []byte("test_output"))
}
