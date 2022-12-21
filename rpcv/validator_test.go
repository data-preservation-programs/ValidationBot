package rpcv

import (
	"bytes"
	"testing"
	"validation-bot/module"
	"validation-bot/module/echo"
	"validation-bot/task"

	"github.com/stretchr/testify/assert"
)

func TestRpcServer_NewRPCServer(t *testing.T) {
	assert := assert.New(t)
	mods := map[task.Type]module.AuditorModule{task.Echo: echo.NewEchoAuditor()}

	validator := NewRPCValidator(ValidatorConfig{Modules: mods})
	assert.NotNil(validator)

	json, err := module.NewJSONB(`{"hello":"world"}`)
	assert.NoError(err)
	assert.NotNil(json)

	reply := module.ValidationResult{}
	input := module.ValidationInput{
		Task: task.Task{
			Type:   task.Echo,
			Target: "target",
		},
		Input: json,
	}

	err = validator.Validate(input, &reply)
	assert.NoError(err)
	result := bytes.NewBuffer(reply.Result.Bytes).String()
	assert.Equal("{\"hello\":\"world\"}", result)
}
