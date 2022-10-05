package dispatcher

import (
	"context"
	"testing"
	"time"
	"validation-bot/module"
	"validation-bot/module/echo"
	"validation-bot/task"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

func TestDispatcher_Remove(t *testing.T) {
	assert := assert.New(t)
	db, err := gorm.Open(postgres.Open("dbname=test"), &gorm.Config{})
	assert.Nil(err)
	assert.NotNil(db)

	mockPublisher := &task.MockPublisher{}

	dper, err := NewDispatcher(Config{
		Db: db,
		Modules: []module.Module{
			echo.Echo{},
		},
		TaskPublisher: mockPublisher,
	})

	assert.Nil(err)
	assert.NotNil(dper)

	tsk := task.Definition{
		Target:          "target",
		Type:            "echo",
		IntervalSeconds: 1,
		Definition:      "2",
		DispatchedTimes: 3,
	}
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

func TestDispatcher_Start(t *testing.T) {
	assert := assert.New(t)
	db, err := gorm.Open(postgres.Open("dbname=test"), &gorm.Config{})
	assert.Nil(err)
	assert.NotNil(db)

	mockPublisher := &task.MockPublisher{}

	dper, err := NewDispatcher(Config{
		Db: db,
		Modules: []module.Module{
			echo.Echo{},
		},
		TaskPublisher: mockPublisher,
		CheckInterval: time.Minute,
	})

	assert.Nil(err)
	assert.NotNil(dper)
}

func TestDispatcher_CreateOneoffTask(t *testing.T) {
	assert := assert.New(t)
	db, err := gorm.Open(postgres.Open("dbname=test"), &gorm.Config{})
	assert.Nil(err)
	assert.NotNil(db)

	mockPublisher := &task.MockPublisher{}
	mockPublisher.On("Publish", mock.Anything, mock.Anything).Return(nil)

	dper, err := NewDispatcher(Config{
		Db: db,
		Modules: []module.Module{
			echo.Echo{},
		},
		TaskPublisher: mockPublisher,
	})

	assert.Nil(err)
	assert.NotNil(dper)

	tsk := task.Definition{
		Target:          "target",
		Type:            "echo",
		IntervalSeconds: 0,
		Definition:      "hello world",
		DispatchedTimes: 0,
	}
	err = dper.Create(context.Background(), &tsk)
	assert.Nil(err)
	mockPublisher.AssertCalled(t, "Publish", mock.Anything, []byte(`{"type":"echo","definition_id":"`+tsk.ID.String()+`","target":"target","input":"hello world"}`))
}

func TestDispatcher_CreateAndList(t *testing.T) {
	assert := assert.New(t)
	db, err := gorm.Open(postgres.Open("dbname=test"), &gorm.Config{})
	assert.Nil(err)
	assert.NotNil(db)

	mockPublisher := &task.MockPublisher{}

	dper, err := NewDispatcher(Config{
		Db: db,
		Modules: []module.Module{
			echo.Echo{},
		},
		TaskPublisher: mockPublisher,
	})

	assert.Nil(err)
	assert.NotNil(dper)

	tsk := task.Definition{
		Target:          "target",
		Type:            "echo",
		IntervalSeconds: 1,
		Definition:      "2",
		DispatchedTimes: 3,
	}
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
