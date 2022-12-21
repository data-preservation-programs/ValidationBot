package auditor

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"net/rpc"
	"os"
	"os/exec"
	"path"
	"strconv"
	"time"
	"validation-bot/module"

	"github.com/google/uuid"
	"github.com/pkg/errors"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
)

type Validator interface {
	Call(ctx context.Context, input module.ValidationInput) (*module.ValidationResult, error)
}

type RPCClient struct {
	log     zerolog.Logger
	baseDir string
	timeout time.Duration
}

type ClientConfig struct {
	BaseDir string
	Timeout time.Duration
}

func NewRPCClient(config ClientConfig) *RPCClient {
	return &RPCClient{
		log:     log.With().Str("role", "rpc.client").Caller().Logger(),
		baseDir: config.BaseDir,
		timeout: config.Timeout,
	}
}

func (r *RPCClient) Call(ctx context.Context, input module.ValidationInput) (*module.ValidationResult, error) {
	uuidStr, err := uuid.NewUUID()
	if err != nil {
		return nil, errors.Wrap(err, "failed to generate uuid")
	}

	dirPath := path.Join(r.baseDir, uuidStr.String())

	err = os.MkdirAll(dirPath, 0755)
	if err != nil {
		return nil, errors.Wrap(err, "failed to create directory")
	}

	// err = exec.CommandContext(ctx, "cp", "../../validation_rpc", dirPath).Run()
	if err != nil {
		return nil, errors.Wrap(err, "failed to copy validation_rpc")
	}

	defer os.RemoveAll(dirPath)

	// otherwise potentially use realpath somehow?
	exePath, err := os.Executable()
	if err != nil {
		return nil, errors.Wrap(err, "failed to get executable path")
	}

	// testdata/someMockSCript -- print out port numbers for tests
	// ${os.Args[0] => program executing, ie validation_bot binary
	botPath := path.Join(exePath, os.Args[0])
	// calls /path/to/ValidationBot/validation_bot validation-rpc
	cmd := exec.CommandContext(ctx, botPath, "validation-rpc")
	cmd.Dir = dirPath
	stdout, _ := cmd.StdoutPipe()

	defer stdout.Close()

	err = cmd.Start()
	if err != nil {
		return nil, errors.Wrap(err, "failed to start validation server")
	}

	defer func() {
		if r := recover(); r != nil {
			log.Error().Err(errors.Errorf("%v", r)).Msg("panic")
			cmd.Process.Kill()
		}
	}()

	reply, err := r.CallValidate(ctx, stdout, input)
	if err != nil {
		return nil, errors.Wrap(err, "failed to call validate")
	}

	select {
	case <-ctx.Done():
		cmd.Process.Kill()
		return nil, errors.New("context cancelled")
	default:
		cmd.Process.Kill()

		return reply, nil
	}
}

func (r *RPCClient) CallValidate(ctx context.Context, stdout io.Reader, input module.ValidationInput) (*module.ValidationResult, error) {
	// listen for rpc server port from stdout
	scanner := bufio.NewScanner(stdout)

	var port int

	for scanner.Scan() {
		var err error
		if port, err = strconv.Atoi(scanner.Text()); err != nil {
			return nil, errors.Wrap(err, "failed to parse port")
		}
	}

	// TODO retry logic to handle weird port things
	client, err := rpc.DialHTTP("tcp", fmt.Sprintf("%s:%d", "localhost", port))
	if err != nil {
		return nil, errors.Wrap(err, "failed to dial http")
	}

	defer client.Close()

	var reply module.ValidationResult

	done := make(chan error, 1)

	go func() {
		select {
		case <-ctx.Done():
			err := ctx.Err()
			done <- err
			client.Close()
		case <-done:
		}
	}()

	err = client.Call("RPCAuditor.Validate", input, &reply)
	if err != nil {
		return nil, errors.Wrap(err, "failed to call rpc")
	}

	select {
	case err = <-done:
	default:
		close(done)
	}

	return &reply, err
}
