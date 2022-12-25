package auditor

import (
	"context"
	"strings"
	"testing"
	"time"

	mock4 "validation-bot/role/auditor/mock"
	mock2 "validation-bot/store/mock"
	mock3 "validation-bot/task/mock"

	"validation-bot/role/trust"

	"validation-bot/module/echo"

	"validation-bot/module"
	"validation-bot/store"

	"validation-bot/task"

	"validation-bot/helper"

	"github.com/google/uuid"
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
	mockClientRPC := &mock4.MockRPCClient{Timeout: 15 * time.Second}
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
			ClientRPC:               mockClientRPC,
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

	definition, err := module.NewJSONB(`{"result":"hello world"}`)
	if err != nil {
		t.Fatal(err)
	}

	validationResults := module.ValidationResult{
		ValidationInput: module.ValidationInput{
			Task: task.Task{
				Type:         mock.Anything,
				DefinitionID: uuid.New(),
				Target:       mock.Anything,
				Tag:          mock.Anything,
				TaskID:       uuid.New(),
			},
			Input: definition,
		},
		Result: definition,
	}

	mockClientRPC.On("CallServer", mock.Anything, mock.Anything, mock.Anything).
		Return(&validationResults, nil)

	mockResultPublisher.On("Publish", mock.Anything, mock.Anything).Return(nil)
	mockPublisherSubscriber.On("Publish", mock.Anything, mock.Anything).Return(nil)
	adt.Start(context.Background())
	time.Sleep(time.Second * 5)

	mockPublisherSubscriber.AssertCalled(t, "Next", mock.Anything)
	mockClientRPC.On("Call", mock.Anything, mock.Anything, mock.Anything).
		Return(&validationResults, nil)
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
