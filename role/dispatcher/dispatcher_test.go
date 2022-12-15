package dispatcher

import (
	"context"
	"fmt"
	"strings"
	"testing"
	"time"

	mock2 "validation-bot/task/mock"

	"validation-bot/helper"
	"validation-bot/module"
	"validation-bot/module/echo"
	"validation-bot/task"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

func createDispatcher(t *testing.T) (*gorm.DB, *Dispatcher, *mock2.MockPublisherSubscriber) {
	assert := assert.New(t)
	db, err := gorm.Open(postgres.Open(helper.PostgresConnectionString), &gorm.Config{})
	assert.Nil(err)
	assert.NotNil(db)
	db.Exec("DELETE FROM definitions")

	mockPublisher := &mock2.MockPublisherSubscriber{}
	mockPublisher.On("Publish", mock.Anything, mock.Anything).Return(nil)

	dper, err := NewDispatcher(
		Config{
			DB: db,
			Modules: map[string]module.DispatcherModule{
				task.Echo: echo.Dispatcher{},
			},
			TaskPublisherSubscriber: mockPublisher,
			CheckInterval:           1 * time.Minute,
			Jitter:                  time.Millisecond * 10,
		},
	)
	assert.Nil(err)
	assert.NotNil(dper)

	err = db.AutoMigrate(&task.Definition{})
	assert.Nil(err)

	return db, dper, mockPublisher
}

func TestDispatcher_Remove(t *testing.T) {
	assert := assert.New(t)
	db, dper, _ := createDispatcher(t)
	tsk := task.Definition{
		Target:          "target",
		Type:            "echo",
		IntervalSeconds: 1,
		DispatchedTimes: 3,
	}
	err := tsk.Definition.Set("2")
	assert.Nil(err)
	db.Create(&tsk)

	found := task.Definition{}
	response := db.Find(&found, tsk.ID)
	assert.Equal(int64(1), response.RowsAffected)
	assert.Equal("target", found.Target)
	err = dper.Remove(context.Background(), found.ID)
	assert.Nil(err)

	found2 := task.Definition{}
	response = db.Find(&found2, tsk.ID)
	assert.Equal(int64(0), response.RowsAffected)
}

func TestDispatcher_additionalJitter(t *testing.T) {
	assert := assert.New(t)
	_, dper, _ := createDispatcher(t)
	jitter := dper.additionalJitter()
	fmt.Println(int64(jitter))
	assert.GreaterOrEqual(jitter, 0*time.Millisecond)
	assert.Less(jitter, 10*time.Millisecond)
}

func TestDispatcher_Start_DispatchMultipleTimes(t *testing.T) {
	assert := assert.New(t)
	db, dper, mockPublisher := createDispatcher(t)
	tsk := &task.Definition{
		Target:          "target",
		Type:            "echo",
		IntervalSeconds: 1,
		DispatchedTimes: 0,
	}
	err := tsk.Definition.Set(`{"message": "hello world"}`)
	assert.Nil(err)
	db.Create(tsk)
	dper.checkInterval = time.Millisecond * 200
	dper.Start(context.Background())
	time.Sleep(5 * time.Second)
	mockPublisher.AssertNumberOfCalls(t, "Publish", 5)
	db.First(tsk, tsk.ID)
	assert.Equal(uint32(5), tsk.DispatchedTimes)
}

func TestDispatcher_Start_DonothingForOneoffTask(t *testing.T) {
	assert := assert.New(t)
	db, dper, mockPublisher := createDispatcher(t)
	tsk := &task.Definition{
		Target:          "target",
		Type:            "echo",
		IntervalSeconds: 0,
		DispatchedTimes: 0,
	}
	err := tsk.Definition.Set("hello world")
	assert.Nil(err)
	db.Create(tsk)
	dper.Start(context.Background())
	time.Sleep(5 * time.Second)

	mockPublisher.AssertNotCalled(t, "Publish", mock.Anything, mock.Anything)
}

func TestDispatcher_CreateOneoffTask(t *testing.T) {
	assert := assert.New(t)
	_, dper, mockPublisher := createDispatcher(t)

	tsk := task.Definition{
		Target:          "target",
		Type:            "echo",
		IntervalSeconds: 0,
		DispatchedTimes: 0,
	}
	err := tsk.Definition.Set(`{"message": "hello world"}`)
	assert.Nil(err)
	err = dper.Create(context.Background(), &tsk)
	assert.Nil(err)
	mockPublisher.AssertCalled(
		t,
		"Publish",
		mock.Anything,
		mock.MatchedBy(
			func(msg []byte) bool {
				return strings.Contains(string(msg), `"input":{"message":"hello world"}`)
			},
		),
	)
}

func TestDispatcher_CreateAndList(t *testing.T) {
	assert := assert.New(t)
	db, dper, _ := createDispatcher(t)

	tsk := task.Definition{
		Target:          "target",
		Type:            "echo",
		IntervalSeconds: 1,
		DispatchedTimes: 3,
	}
	err := tsk.Definition.Set("2")
	assert.Nil(err)
	err = dper.Create(context.Background(), &tsk)
	assert.Nil(err)

	found := task.Definition{}
	response := db.Find(&found, tsk.ID)
	assert.Equal(int64(1), response.RowsAffected)
	assert.Equal("target", found.Target)

	list, err := dper.List(context.Background())
	assert.Nil(err)

	hasFound := false
	for _, tsk := range list {
		if tsk.ID == found.ID {
			hasFound = true
		}
	}

	assert.True(hasFound)
}
