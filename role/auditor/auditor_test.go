package auditor

import (
	"context"
	"strings"
	"testing"
	"time"
	mock2 "validation-bot/store/mock"
	mock3 "validation-bot/task/mock"

	"validation-bot/role/trust"

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
	_, _, dispatcherPeerID := helper.GeneratePeerID(t)
	_, _, auditorPeerID := helper.GeneratePeerID(t)
	mockResultPublisher := &mock2.MockPublisher{}
	mockPublisherSubscriber := &mock3.MockPublisherSubscriber{}
	inmemoryStore := store.InMemoryStore{
		Storage: make([][]byte, 0),
	}
	trustManager := trust.NewManager(
		[]peer.ID{dispatcherPeerID},
		&inmemoryStore,
		time.Second,
		time.Minute,
	)
	err := trust.ModifyPeers(
		context.Background(),
		&inmemoryStore,
		&inmemoryStore,
		trust.Create,
		dispatcherPeerID,
		[]peer.ID{auditorPeerID},
		time.Second,
	)
	assert.NoError(err)
	adt, err := NewAuditor(
		Config{
			PeerID:                  auditorPeerID,
			TrustManager:            trustManager,
			ResultPublisher:         mockResultPublisher,
			TaskPublisherSubscriber: mockPublisherSubscriber,
			Modules:                 map[task.Type]module.AuditorModule{task.Echo: echo.NewEchoAuditor()},
			BiddingWait:             3 * time.Second,
		},
	)
	assert.Nil(err)
	assert.NotNil(adt)
	mockPublisherSubscriber.On("Next", mock.Anything).
		After(time.Duration(time.Second)).Return(
		&dispatcherPeerID,
		[]byte(`{"type":"echo","taskId":"a4d90653-de5d-4ef4-b4dd-4b0818dd73a3","definitionId":"d17e7152-af60-494c-9391-1270293d2c08","target":"target","input":"hello world"}`),
		nil,
	).Once()
	mockPublisherSubscriber.On("Next", mock.Anything).
		After(time.Duration(time.Second)).Return(
		&auditorPeerID,
		[]byte(`{"type":"bidding","taskId":"a4d90653-de5d-4ef4-b4dd-4b0818dd73a3","value":100}`),
		nil,
	).Once()
	mockPublisherSubscriber.On("Next", mock.Anything).
		After(time.Duration(time.Hour)).Return(
		&auditorPeerID,
		[]byte(`{"type":"bidding","taskId":"a4d90653-de5d-4ef4-b4dd-4b0818dd73a3","value":100}`),
		nil,
	)
	mockResultPublisher.On("Publish", mock.Anything, mock.Anything).Return(nil)
	mockPublisherSubscriber.On("Publish", mock.Anything, mock.Anything).Return(nil)
	adt.Start(context.Background())
	time.Sleep(time.Second * 5)

	mockPublisherSubscriber.AssertCalled(t, "Next", mock.Anything)
	mockResultPublisher.AssertCalled(
		t,
		"Publish",
		mock.Anything,
		mock.MatchedBy(
			func(data []byte) bool {
				return strings.Contains(string(data), `"result":"hello world"`)
			},
		),
	)
}
