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

	"github.com/pkg/errors"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
)

type IRPCClient interface {
	Call(ctx context.Context, input module.ValidationInput) (*module.ValidationResult, error)
	CallValidate(ctx context.Context, stdout io.Reader, input module.ValidationInput) (*module.ValidationResult, error)
}

type RPCClient struct {
	IRPCClient
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
	return r.call(ctx, input, r.CallValidate)
}

// unexported call with callValidate - replace with anon function for testing purposes
func (r *RPCClient) call(
	ctx context.Context,
	input module.ValidationInput,
	callValidate func(context.Context, io.Reader, module.ValidationInput) (*module.ValidationResult, error),
) (*module.ValidationResult, error) {
	dir, err := os.MkdirTemp(r.baseDir, "validation_rpc")
	if err != nil {
		return nil, errors.Wrap(err, "failed to create directory")
	}

	defer os.RemoveAll(dir)

	// otherwise potentially use realpath somehow?
	exePath, err := os.Executable()
	if err != nil {
		return nil, errors.Wrap(err, "failed to get executable path")
	}

	// ${os.Args[0] => name of the current program executing, ie validation_bot binary
	botPath := path.Join(exePath, os.Args[0])
	fmt.Println("botPath: ", botPath)
	// calls /path/to/ValidationBot/validation_bot validation-rpc
	cmd := exec.CommandContext(ctx, botPath, "validation-rpc")
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

	reply, err := callValidate(ctx, stdout, input)
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
		fmt.Print("scanner text:", scanner.Text())
		if _port, err := strconv.Atoi(scanner.Text()); err != nil {
			return nil, errors.Wrap(err, "failed to parse port")
		} else {
			port = _port
		}
	}

	fmt.Printf("port: %d\n", port)

	// TODO retry logic to handle weird port things
	// wait 100ms for port to be ready with retry backup
	conn := fmt.Sprintf("%s:%d", "0.0.0.0", port)
	fmt.Printf("conn: %s \n", conn)
	client, err := rpc.DialHTTP("tcp", conn)
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

	err = client.Call("RPCValidator.Validate", input, &reply)
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
