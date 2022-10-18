package observer

import (
	"context"
	"fmt"
	"testing"
	"time"

	"validation-bot/module"
	"validation-bot/store"
	"validation-bot/test"

	"github.com/google/uuid"
	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

const (
	testPeerId = "12D3KooWG8tR9PHjjXcMknbNPVWT75BuXXA2RaYx3fMwwg2oPZXd"
	testTarget = "test_target"
)

func TestObserverStart(t *testing.T) {
	assert := assert.New(t)
	testDefinitionUUID, err := uuid.NewUUID()
	assert.Nil(err)
	testDefinitionId := testDefinitionUUID.String()
	db, err := gorm.Open(postgres.Open(test.PostgresConnectionString), &gorm.Config{})
	assert.Nil(err)
	assert.NotNil(db)

	subscriber := new(store.MockSubscriber)
	obs, err := NewObserver(db, subscriber, []peer.ID{
		peer.ID(testPeerId),
	})
	assert.Nil(err)
	assert.NotNil(obs)

	mockChan := make(chan store.Entry)
	var writeOnly <-chan store.Entry = mockChan
	subscriber.On("Subscribe",
		mock.Anything,
		mock.Anything,
		mock.Anything).Return(writeOnly, nil)
	testOutput := uuid.New().String()
	go func() {
		mockChan <- store.Entry{
			Previous: nil,
			Message: []byte(fmt.Sprintf(`{"type":"echo","definitionId":"%s","target":"%s", "result": {"message": "%s"}}`,
				testDefinitionId, testTarget, testOutput)),
		}
	}()
	errChan := obs.Start(context.Background())
	assert.NotNil(errChan)
	select {
	case err := <-errChan:
		assert.Fail("unexpected error", err)
	case <-time.After(2 * time.Second):
	}

	var found []module.ValidationResult
	db.Model(&module.ValidationResultModel{}).Where("definition_id = ?", testDefinitionId).Find(&found)
	assert.Equal(1, len(found))
	assert.Equal(fmt.Sprintf(`{"message": "%s"}`, testOutput), string(found[0].Result.Bytes))
}
