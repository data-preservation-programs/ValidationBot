package auditor

import (
	"context"
	"fmt"
	"testing"
	"time"
	"validation-bot/module"
	"validation-bot/task"

	"github.com/stretchr/testify/assert"
)

func TestRPCClient__Validate(t *testing.T) {
	assert := assert.New(t)
	// rpcServer := rpcv.NewRPCValidator(
	// 	rpcv.ValidatorConfig{
	// 		Modules: map[task.Type]module.AuditorModule{task.Echo: echo.NewEchoAuditor()},
	// 	},
	// )
	// assert.NotNil(rpcServer)

	ctx := context.Background()
	// rpcServer.Start(ctx)

	rpcClient := NewRPCClient(
		ClientConfig{
			BaseDir: "/tmp",
			Timeout: 2 * time.Minute,
		})

	definition, err := module.NewJSONB(`{"hello":"world"}`)
	input := module.ValidationInput{
		Task: task.Task{
			Type:   task.Echo,
			Target: "target",
		},
		Input: definition,
	}
	// input := new(module.ValidationInput)
	// err = json.Unmarshal(tsk, input)
	rpcClient.Call(ctx, input)

	assert.NotNil(rpcClient)

	// mockModule := &mockModule{}
	// rpcClient.On("Validate", mock.Anything, mock.Anything).Return(&module.ValidationResult{}, nil)
	// rpcClient := NewRPCClient("localhost:1234", mockModule)
	result, err := rpcClient.Call(context.Background(), input)
	assert.NoError(err)

	fmt.Print("result: ", result)
}
