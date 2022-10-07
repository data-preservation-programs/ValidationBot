package auditor

import (
	"context"
	"testing"
	"time"

	"validation-bot/module"
	"validation-bot/module/echo"
	"validation-bot/store"

	"validation-bot/task"

	"validation-bot/test"

	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

func TestAuditor_Start(t *testing.T) {
	assert := assert.New(t)
	_, _, peerId := test.GeneratePeerID(t)
	mockPublisher := &store.MockPublisher{}
	mockSubscriber := &task.MockSubscriber{}
	adt, err := NewAuditor(Config{
		TrustedPeers:    []peer.ID{peerId},
		ResultPublisher: mockPublisher,
		TaskSubscriber:  mockSubscriber,
		Modules:         []module.Module{echo.Echo{}},
	})
	assert.Nil(err)
	assert.NotNil(adt)
	mockSubscriber.On("Next", mock.Anything).
		After(time.Duration(time.Second)).Return(&peerId, []byte(`{"type":"echo","definitionId":"d17e7152-af60-494c-9391-1270293d2c08","target":"target","input":"hello world"}`), nil)
	mockPublisher.On("Publish", mock.Anything, mock.Anything).Return(nil)
	errChan := adt.Start(context.Background())
	select {
	case err := <-errChan:
		assert.Fail("unexpected error", err)
	case <-time.After(time.Duration(2 * time.Second)):
	}

	mockSubscriber.AssertCalled(t, "Next", mock.Anything)
	mockPublisher.AssertCalled(t, "Publish", mock.Anything, []byte(`{"type":"echo","definitionId":"d17e7152-af60-494c-9391-1270293d2c08","target":"target","output":"hello world"}`))
}
