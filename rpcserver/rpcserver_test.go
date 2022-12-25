package rpcserver

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

	rpcServer := NewRPCServer(Config{Modules: mods})
	assert.NotNil(rpcServer)

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

	err = rpcServer.Validate(input, &reply)
	assert.NoError(err)
	result := bytes.NewBuffer(reply.Result.Bytes).String()
	assert.Equal("{\"hello\":\"world\"}", result)
}
