package auditor

import (
	"context"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"testing"
	"time"
	"validation-bot/module"
	"validation-bot/module/echo"
	"validation-bot/rpcserver"
	"validation-bot/task"

	"github.com/google/uuid"
	"github.com/pkg/errors"
	"github.com/stretchr/testify/assert"
)

func TestRPCClient__Validate(t *testing.T) {
	rpcServer := rpcserver.NewRPCServer(
		rpcserver.Config{
			Modules: map[task.Type]module.AuditorModule{task.Echo: echo.NewEchoAuditor()},
		},
	)

	assert.NotNil(t, rpcServer)

	type stdout = string

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	done := make(chan struct{})

	wd, err := os.Getwd()
	if err != nil {
		panic(err)
	}
	absPath, err := filepath.Abs(filepath.Dir(wd))
	if err != nil {
		panic(err)
	}

	dir, err := os.MkdirTemp(absPath, "validation-rpc-test")
	if err != nil {
		assert.Fail(t, errors.Wrap(err, "failed to create temp dir").Error())
	}

	defer os.RemoveAll(dir)

	go func() {
		defer close(done)
		err := rpcServer.Start(ctx, 1234, dir)
		assert.NoError(t, err)
	}()

	// wait for server to start
	time.Sleep(250 * time.Millisecond)

	rpcClient := NewClientRPC(
		ClientConfig{
			BaseDir: dir,
			Timeout: 2 * time.Minute,
		})

	definition, err := module.NewJSONB(`{"hello":"world"}`)
	assert.NoError(t, err)

	input := module.ValidationInput{
		Task: task.Task{
			Type:   task.Echo,
			Target: "target",
		},
		Input: definition,
	}

	result, err := rpcClient.Validate(context.Background(), 1234, input)

	assert.NoError(t, err)
	assert.NotNil(t, result)

	if _, err = os.Stat(dir); errors.Is(err, fs.ErrExist) {
		os.RemoveAll(dir)
		assert.Fail(t, fmt.Sprintf("%s still exists:", dir))
	}

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

	dir, err := os.MkdirTemp(absPath, "validation-rpc-test")
	if err != nil {
		assert.Fail(t, errors.Wrap(err, "failed to create temp dir").Error())
	}

	defer os.RemoveAll(dir)

	rpcClient := NewClientRPC(ClientConfig{
		BaseDir:  dir,
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

	result, err := rpcClient.CallServer(ctx, dir, tsk)
	fmt.Printf("result: %v\n", result)
	fmt.Printf("err: %v\n", err)

	if _, err := os.Stat(dir); errors.Is(err, fs.ErrExist) {
		os.RemoveAll(dir)
		assert.Fail(t, fmt.Sprintf("%s still exists:", dir))
	}

	assert.NoError(t, err)
	assert.Equal(t, result.Result, json)
}
