package echo

import (
	"testing"
	"validation-bot/module"
	"validation-bot/task"

	"github.com/stretchr/testify/assert"
)

func TestEcho_Validate(t *testing.T) {
	assert := assert.New(t)
	echo := Auditor{}
	definition, err := module.NewJSONB(`{"hello":"world"}`)
	assert.Nil(err)
	input := module.ValidationInput{
		Task: task.Task{
			Type:   task.Echo,
			Target: "target",
		},
		Input: definition,
	}
	result, err := echo.Validate(nil, input)
	assert.Nil(err)
	assert.Equal(input.Task, result.Task)
	assert.Equal(input.Input, result.Result)
}
