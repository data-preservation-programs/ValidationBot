package echo

import (
	"testing"
	"validation-bot/task"

	"github.com/stretchr/testify/assert"
)

func TestEcho_GetTasks(t *testing.T) {
	assert := assert.New(t)
	echo := Echo{}
	tsk := task.Definition{
		Target:          "target",
		Type:            "echo",
		IntervalSeconds: 0,
		Definition:      "definition",
		DispatchedTimes: 0,
	}
	tasks, err := echo.GetTasks([]task.Definition{tsk})
	assert.Nil(err)
	assert.Equal(1, len(tasks))
	bytes := tasks[tsk]
	assert.Nil(err)
	assert.Equal(`{"type":"echo","definition_id":"00000000-0000-0000-0000-000000000000","target":"target","input":"definition"}`, string(bytes))
}

func TestEcho_GetTask(t *testing.T) {
	assert := assert.New(t)
	echo := Echo{}
	tsk := task.Definition{
		Target:          "target",
		Type:            "echo",
		IntervalSeconds: 0,
		Definition:      "definition",
		DispatchedTimes: 0,
	}
	task, err := echo.GetTask(tsk)
	assert.Nil(err)
	bytes := task
	assert.Nil(err)
	assert.Equal(`{"type":"echo","definition_id":"00000000-0000-0000-0000-000000000000","target":"target","input":"definition"}`, string(bytes))
}

func TestEcho_Validate(t *testing.T) {
	assert := assert.New(t)
	echo := Echo{}
	bytes, err := echo.Validate(nil, []byte(`{"type":"echo","definition_id":"00000000-0000-0000-0000-000000000000","target":"target","input":"definition"}`))
	assert.Nil(err)
	assert.Equal(`{"type":"echo","definition_id":"00000000-0000-0000-0000-000000000000","target":"target","output":"definition"}`, string(bytes))
}
