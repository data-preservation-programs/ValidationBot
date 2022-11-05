package module

import (
	"testing"

	"validation-bot/task"

	"github.com/stretchr/testify/assert"
)

func TestSimpleDispatcherModule_GetTasks(t *testing.T) {
	assert := assert.New(t)
	definition, err := NewJSONB(`{"hello":"world"}`)
	assert.Nil(err)
	m := SimpleDispatcher{}
	tsk := task.Definition{
		Target:          "target",
		Type:            "echo",
		IntervalSeconds: 0,
		Definition:      definition,
		DispatchedTimes: 0,
		Tag:             "tag",
	}
	tasks, err := m.GetTasks([]task.Definition{tsk})
	assert.Nil(err)
	assert.Equal(1, len(tasks))
	bytes := tasks[tsk.ID]
	assert.Nil(err)
	assert.Equal("echo", bytes.Type)
	assert.Equal(`{"hello":"world"}`, string(bytes.Input.Bytes))
	assert.Equal("target", bytes.Target)
	assert.Equal("tag", bytes.Tag)
}

func TestSimpleDispatcherModule_GetTask(t *testing.T) {
	assert := assert.New(t)
	definition, err := NewJSONB(`{"hello":"world"}`)
	assert.Nil(err)
	m := SimpleDispatcher{}
	tsk := task.Definition{
		Target:          "target",
		Type:            "echo",
		IntervalSeconds: 0,
		Definition:      definition,
		DispatchedTimes: 0,
	}
	task, err := m.GetTask(tsk)
	assert.Nil(err)
	bytes := task
	assert.Nil(err)
	assert.Equal("echo", bytes.Type)
	assert.Equal(`{"hello":"world"}`, string(bytes.Input.Bytes))
	assert.Equal("target", bytes.Target)
}
