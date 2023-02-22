package auditor

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
	"validation-bot/module"
	"validation-bot/module/echo"
	"validation-bot/rpcserver"
	"validation-bot/task"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
)

func TestRPCClient__Validate(t *testing.T) {
	assert := assert.New(t)

	rpcServer := rpcserver.NewRPCServer(
		rpcserver.Config{
			Modules: map[task.Type]module.AuditorModule{task.Echo: echo.NewEchoAuditor()},
		},
	)

	assert.NotNil(rpcServer)

	type stdout = string

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	done := make(chan struct{})

	go func() {
		defer close(done)
		err := rpcServer.Start(ctx, 1234)
		assert.NoError(err)
	}()

	// wait for server to start
	time.Sleep(250 * time.Millisecond)

	rpcClient := NewClientRPC(
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
	result, err := rpcClient.Validate(context.Background(), portStdOut, input)

	assert.NoError(err)
	assert.NotNil(result)

	fmt.Printf("result: %v\n", result)

	cancel()
	<-done
}

func TestRPCClient_Call__FeatureTest(t *testing.T) {
	wd, err := os.Getwd()
	if err != nil {
		panic(err)
	}
	absPath, err := filepath.Abs(filepath.Dir(wd))
	if err != nil {
		panic(err)
	}

	path := filepath.Join(absPath, "../")

	rpcClient := NewClientRPC(ClientConfig{
		BaseDir:  "/tmp",
		Timeout:  2 * time.Minute,
		ExecPath: path,
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

	result, err := rpcClient.CallServer(ctx, tsk)
	fmt.Printf("result: %v\n", result)
	fmt.Printf("err: %v\n", err)
	assert.NoError(t, err)
	assert.Equal(t, result.Result, json)
}
