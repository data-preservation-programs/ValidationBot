package auditor

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"testing"
	"time"
	"validation-bot/module"
	"validation-bot/module/echo"
	"validation-bot/rpcv"
	"validation-bot/task"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
)

var serveOnce sync.Once

func TestRPCClient__CallValidate(t *testing.T) {
	assert := assert.New(t)
	rpcServer := rpcv.NewRPCValidator(
		rpcv.ValidatorConfig{
			Modules: map[task.Type]module.AuditorModule{task.Echo: echo.NewEchoAuditor()},
		},
	)
	assert.NotNil(rpcServer)

	type portNumber = int
	type stdout = string

	go func() {
		ctx := context.Background()
		serveOnce.Do(func() { rpcServer.Start(ctx, 1234) })
	}()

	rpcClient := NewRPCClient(
		ClientConfig{
			BaseDir: "/tmp",
			Timeout: 2 * time.Minute,
		})

	definition, err := module.NewJSONB(`{"hello":"world"}`)
	assert.NoError(err)

	input := module.ValidationInput{
		Task: task.Task{
			Type:   task.Echo,
			Target: "target",
		},
		Input: definition,
	}

	portStdOut := strings.NewReader("1234")
	result, err := rpcClient.CallValidate(context.Background(), portStdOut, input)

	assert.NoError(err)
	assert.NotNil(result)

	// mockModule := &mockModule{}
	// rpcClient.On("Validate", mock.Anything, mock.Anything).Return(&module.ValidationResult{}, nil)
	// rpcClient := NewRPCClient("localhost:1234", mockModule)
	// result, err := rpcClient.CallValidate(context.Background(), input)
	assert.NoError(err)

	fmt.Print("result: ", result)
}

func TestRPCClient_Call(t *testing.T) {
	rpcClient := NewRPCClient(
		ClientConfig{
			BaseDir: "/tmp",
			Timeout: 30 * time.Second,
		})

	id := uuid.New()
	json, err := module.NewJSONB(`{"hello":"world"}`)
	assert.NoError(t, err)

	tsk := module.ValidationInput{
		Task: task.Task{
			Type:         task.Echo,
			DefinitionID: id,
			Target:       "target",
			Tag:          "tag",
			TaskID:       id,
		},
		Input: json,
	}

	// TODO mock CallValidate
	rpcClient.Call(context.Background(), tsk)
}
