package auditor

import (
	"context"
	"fmt"
	"io"
	"strings"
	"testing"
	"time"
	"validation-bot/module"
	"validation-bot/module/echo"
	"validation-bot/rpcv"
	"validation-bot/task"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
)

func TestRPCClient__Call(t *testing.T) {
	// TODO:
}

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

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	done := make(chan struct{})

	go func() {
		defer close(done)
		err := rpcServer.Start(ctx, 1234)
		assert.NoError(err)
	}()

	time.Sleep(1 * time.Second)

	rpcClient := NewRPCClient(
		ClientConfig{
			BaseDir: "/tmp",
			Timeout: 2 * time.Minute,
		})

	definition, err := module.NewJSONB(`{"hello":"world"}`)
	assert.NoError(err)

	// TODO cant find echo module
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

	assert.NoError(err)

	fmt.Print("result: ", result)

	cancel()
	<-done
}

func TestRPCClient_Call__startsServer(t *testing.T) {
	rpcClient := NewRPCClient(ClientConfig{
		BaseDir: "/tmp",
		Timeout: 2 * time.Minute,
	})

	ctx := context.Background()

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

	// mock call
	cValidate := func(ctx context.Context, stdout io.Reader, tsk module.ValidationInput) (*module.ValidationResult, error) {
		return &module.ValidationResult{}, nil
	}

	rpcClient.call(ctx, tsk, cValidate)

	// mockClient.Call(context.Background(), tsk)
	// rpcClient.On("CallValidate", mock.Anything, mock.Anything).Return(&module.ValidationResult{}, nil)
}
