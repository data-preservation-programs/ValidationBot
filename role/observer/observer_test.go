package observer

import (
	"context"
	"fmt"
	"testing"
	"time"
	"validation-bot/module"
	"validation-bot/module/echo"
	"validation-bot/store"

	"github.com/google/uuid"
	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

const (
	testPeerId       = "12D3KooWG8tR9PHjjXcMknbNPVWT75BuXXA2RaYx3fMwwg2oPZXd"
	testDefinitionId = "92cb6e41-1019-4751-b822-ba3b92532a2b"
	testTarget       = "test_target"
)

func TestObserverStart(t *testing.T) {
	assert := assert.New(t)
	db, err := gorm.Open(postgres.Open("dbname=test"), &gorm.Config{})
	assert.Nil(err)
	assert.NotNil(db)

	subscriber := new(store.MockSubscriber)
	obs, err := NewObserver(db, subscriber, []peer.ID{
		peer.ID(testPeerId),
	}, []module.Module{
		&echo.Echo{},
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
			Message: []byte(fmt.Sprintf("{\"type\":\"echo\",\"definition_id\":\"%s\",\"target\":\"%s\", \"output\": \"%s\"}",
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

	var found []echo.EchoResult
	db.Model(&echo.EchoResult{}).Where("output = ?", testOutput).Find(&found)
	assert.Equal(1, len(found))
	assert.Equal(testOutput, found[0].ResultContent.Output)
}
