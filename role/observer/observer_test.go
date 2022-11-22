package observer

import (
	"context"
	"fmt"
	"testing"
	"time"

	"validation-bot/role/trust"

	"validation-bot/helper"
	"validation-bot/module"
	"validation-bot/store"

	"github.com/google/uuid"
	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/stretchr/testify/assert"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

const (
	testTarget = "test_target"
	testCid    = "bafkreig4bdyaaedbcqy7ysylkbwkomo43aax223btxefxfcal4aiz6iw6e"
)

func TestObserverStart(t *testing.T) {
	assert := assert.New(t)
	trustStore := store.InMemoryStore{
		Storage: [][]byte{},
	}
	resultStore := store.InMemoryStore{
		Storage: [][]byte{},
	}

	_, _, trustor := helper.GeneratePeerID(t)
	_, _, newPeer1 := helper.GeneratePeerID(t)
	err := trust.ModifyPeers(
		context.Background(),
		&trustStore,
		&trustStore,
		trust.Create,
		trustor,
		[]peer.ID{newPeer1},
		time.Second,
	)
	assert.NoError(err)
	testDefinitionUUID, err := uuid.NewUUID()
	assert.Nil(err)
	testDefinitionId := testDefinitionUUID.String()
	db, err := gorm.Open(postgres.Open(helper.PostgresConnectionString), &gorm.Config{})
	assert.Nil(err)
	assert.NotNil(db)
	err = db.AutoMigrate(&module.ValidationResultModel{})
	assert.Nil(err)
	manager := trust.NewManager([]peer.ID{trustor}, &trustStore, time.Second, time.Second)

	db.Create(
		&module.ValidationResultModel{
			Cid:    testCid,
			PeerID: newPeer1.String(),
		},
	)
	obs, err := NewObserver(
		db, manager, &resultStore,
	)
	assert.Nil(err)
	assert.NotNil(obs)
	ctx := context.TODO()
	testOutput := uuid.New().String()
	err = resultStore.Publish(
		ctx, []byte(fmt.Sprintf(
			`{"type":"echo","definitionId":"%s","target":"%s", "result": {"message": "%s"}}`,
			testDefinitionId, testTarget, testOutput,
		)),
	)
	assert.Nil(err)

	obs.Start(ctx)
	time.Sleep(2 * time.Second)

	var found []module.ValidationResult
	db.Model(&module.ValidationResultModel{}).Where("definition_id = ?", testDefinitionId).Find(&found)
	assert.Equal(1, len(found))
	assert.Equal(fmt.Sprintf(`{"message": "%s"}`, testOutput), string(found[0].Result.Bytes))
}
