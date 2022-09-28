package auditor

import (
	"context"
	"strconv"
	"testing"
	"time"

	"validation-bot/task"

	"validation-bot/pubsubtest"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

const (
	TestType task.Type = "test"
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
	private, _, peerID := pubsubtest.GeneratePeerID(t)
	topic, pubPort, subPort := "test", 5556, 5557
	mockPublisher := new(MockPublisher)
	mockPublisher.On("Publish", mock.Anything, mock.Anything).Return(nil)

	auditor, err := NewAuditor(context.Background(), Config{
		PrivateKey:   private,
		PeerID:       peerID,
		ListenAddr:   "/ip4/0.0.0.0/tcp/" + strconv.Itoa(subPort),
		topicName:    topic,
		TrustedPeers: []string{},
		publisher:    mockPublisher,
	})
	assert.Nil(err)

	mockValidator := new(MockValidator)
	auditor.validators[TestType] = mockValidator
	mockValidator.On("Validate", mock.Anything, mock.Anything).Return([]byte("test_output"), nil)

	ctx, cancel := context.WithTimeout(context.Background(), time.Second*3)
	defer cancel()
	task := []byte("{\"type\":\"test\",\"target\":\"target\",\"input\":\"hello\"}")
	go func(ctx context.Context) {
		pubsubtest.PublishTask(ctx, t, pubPort, subPort, peerID, topic, task)
		<-ctx.Done()
	}(ctx)
	auditor.Start(ctx)

	// Verify the test validator has been called with the test task
	mockValidator.AssertCalled(t, "Validate", mock.Anything, task)
	// Verify the message has been published
	mockPublisher.AssertCalled(t, "Publish", mock.Anything, []byte("test_output"))
}
