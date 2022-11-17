package auditor

import (
	"context"
	"testing"
	"time"

	"validation-bot/module/echo"

	"validation-bot/module"
	"validation-bot/store"

	"validation-bot/task"

	"validation-bot/helper"

	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

func TestAuditor_Start(t *testing.T) {
	assert := assert.New(t)
	_, _, peerId := helper.GeneratePeerID(t)
	mockPublisher := &store.MockPublisher{}
	mockSubscriber := &task.MockSubscriber{}
	adt, err := NewAuditor(
		Config{
			TrustedDispatcherPeers: []peer.ID{peerId},
			ResultPublisher:        mockPublisher,
			TaskSubscriber:         mockSubscriber,
			Modules:                map[task.Type]module.AuditorModule{task.Echo: echo.NewEchoAuditor()},
		},
	)
	assert.Nil(err)
	assert.NotNil(adt)
	mockSubscriber.On("Next", mock.Anything).
		After(time.Duration(time.Second)).Return(
		&peerId,
		[]byte(`{"type":"echo","definitionId":"d17e7152-af60-494c-9391-1270293d2c08","target":"target","input":"hello world"}`),
		nil,
	)
	mockPublisher.On("Publish", mock.Anything, mock.Anything).Return(nil)
	adt.Start(context.Background())
	time.Sleep(time.Second * 2)

	mockSubscriber.AssertCalled(t, "Next", mock.Anything)
	mockPublisher.AssertCalled(
		t,
		"Publish",
		mock.Anything,
		[]byte(`{"type":"echo","definitionId":"d17e7152-af60-494c-9391-1270293d2c08","target":"target","input":"hello world","result":"hello world"}`),
	)
}
