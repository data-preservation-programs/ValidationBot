package observer

import (
	"context"
	"fmt"
	"testing"
	"time"

	"validation-bot/helper"
	"validation-bot/module"
	"validation-bot/store"

	"github.com/google/uuid"
	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

const (
	testTarget = "test_target"
	testCid    = "bafkreig4bdyaaedbcqy7ysylkbwkomo43aax223btxefxfcal4aiz6iw6e"
)

func TestObserverStart(t *testing.T) {
	assert := assert.New(t)
	_, _, testPeerId1 := helper.GeneratePeerID(t)
	_, _, testPeerId2 := helper.GeneratePeerID(t)
	testDefinitionUUID, err := uuid.NewUUID()
	assert.Nil(err)
	testDefinitionId := testDefinitionUUID.String()
	db, err := gorm.Open(postgres.Open(helper.PostgresConnectionString), &gorm.Config{})
	assert.Nil(err)
	assert.NotNil(db)

	subscriber := new(store.MockSubscriber)
	err = db.AutoMigrate(&module.ValidationResultModel{})
	assert.Nil(err)
	db.Create(
		&module.ValidationResultModel{
			Cid:    testCid,
			PeerID: testPeerId1.String(),
		},
	)
	obs, err := NewObserver(
		db, subscriber, []peer.ID{
			testPeerId1,
			testPeerId2,
		},
	)
	assert.Nil(err)
	assert.NotNil(obs)

	mockChan := make(chan store.Entry)
	var writeOnly <-chan store.Entry = mockChan
	subscriber.On(
		"Subscribe",
		mock.Anything,
		mock.Anything,
		mock.Anything,
		mock.Anything,
	).Return(writeOnly, nil)
	testOutput := uuid.New().String()
	go func() {
		mockChan <- store.Entry{
			Previous: nil,
			Message: []byte(fmt.Sprintf(
				`{"type":"echo","definitionId":"%s","target":"%s", "result": {"message": "%s"}}`,
				testDefinitionId, testTarget, testOutput,
			)),
		}
	}()
	obs.trustedAuditorPeers[testPeerId1] = struct{}{}
	obs.trustedAuditorPeers[testPeerId2] = struct{}{}
	go obs.downloadEntriesForAuditorPeer(context.Background(), testPeerId1)
	go obs.downloadEntriesForAuditorPeer(context.Background(), testPeerId2)
	time.Sleep(2 * time.Second)

	var found []module.ValidationResult
	db.Model(&module.ValidationResultModel{}).Where("definition_id = ?", testDefinitionId).Find(&found)
	assert.Equal(1, len(found))
	assert.Equal(fmt.Sprintf(`{"message": "%s"}`, testOutput), string(found[0].Result.Bytes))
}
