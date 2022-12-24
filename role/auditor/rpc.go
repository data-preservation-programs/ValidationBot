package auditor

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"net/rpc"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"time"
	"validation-bot/module"

	"github.com/pkg/errors"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
)

type IClientRPC interface {
	CallServer(ctx context.Context, input module.ValidationInput) (*module.ValidationResult, error)
	Validate(
		ctx context.Context,
		stdout io.Reader,
		input module.ValidationInput,
	) (*module.ValidationResult, error)
	GetTimeout() time.Duration
}

type ClientRPC struct {
	log      zerolog.Logger
	baseDir  string
	Timeout  time.Duration
	execPath string
	Cmd      *exec.Cmd
}

type ClientConfig struct {
	BaseDir  string
	Timeout  time.Duration
	ExecPath string
}

func NewClientRPC(config ClientConfig) *ClientRPC {
	return &ClientRPC{
		log:      log.With().Str("role", "rpc.client").Caller().Logger(),
		baseDir:  config.BaseDir,
		Timeout:  config.Timeout,
		execPath: config.ExecPath,
	}
}

func (r *ClientRPC) CallServer(ctx context.Context, input module.ValidationInput) (*module.ValidationResult, error) {
	return r.callServer(ctx, input, r.Validate)
}

func (r *ClientRPC) GetTimeout() time.Duration {
	return r.Timeout
}

// unexported callServer - replace with anon function for testing purposes.
func (r *ClientRPC) callServer(
	ctx context.Context,
	input module.ValidationInput,
	validate func(context.Context, io.Reader, module.ValidationInput) (*module.ValidationResult, error),
) (*module.ValidationResult, error) {
	dir, err := os.MkdirTemp(r.baseDir, "validation_rpc")
	if err != nil {
		return nil, errors.Wrap(err, "failed to create directory")
	}

	// err = os.Chmod(dir, 0777)
	if err != nil {
		return nil, errors.Wrap(err, "failed to change permissions")
	}

	defer os.RemoveAll(dir)

	absdir, err := filepath.Abs(dir)
	fmt.Println("Absolute dir", absdir)
	fmt.Println("exec path", r.execPath)

	// calls /path/to/ValidationBot/validation_bot validation-rpc
	r.Cmd = exec.CommandContext(ctx, dir, "validation-rpc")
	r.Cmd.Dir = absdir
	r.Cmd.Path = r.execPath + "validation_bot"
	stdout, _ := r.Cmd.StdoutPipe()

	defer stdout.Close()

	err = r.Cmd.Start()
	if err != nil {
		return nil, errors.Wrap(err, "failed to start validation server")
	}

	defer func() {
		if rec := recover(); rec != nil {
			log.Error().Err(errors.Errorf("%v", rec)).Msg("panic")

			err = r.Cmd.Process.Kill()
			if err != nil {
				log.Error().Err(err).Msg("failed to kill process")
			}
		}
	}()

	fmt.Printf("cmd: %v\n", r.Cmd)

	reply, err := validate(ctx, stdout, input)
	if err != nil {
		return nil, errors.Wrap(err, "failed to call validate")
	}

	select {
	case <-ctx.Done():
		err = r.Cmd.Process.Kill()
		if err != nil {
			return nil, errors.Wrap(err, "failed to kill process")
		}
		return nil, errors.New("context cancelled")
	default:
		err = r.Cmd.Process.Kill()
		if err != nil {
			return nil, errors.Wrap(err, "failed to kill process")
		}

		return reply, nil
	}
}

func (r *ClientRPC) Validate(
	ctx context.Context,
	stdout io.Reader,
	input module.ValidationInput,
) (*module.ValidationResult, error) {
	// listen for rpc server port from stdout
	scanner := bufio.NewScanner(stdout)

	var port int
	scans := 1

	for scanner.Scan() {
		if _port, err := strconv.Atoi(scanner.Text()); err != nil {
			scans += 1
			if scans > 3 {
				return nil, errors.Wrap(err, "failed to parse port")
			}
			time.Sleep(200 * time.Millisecond)
		} else {
			port = _port
			break
		}
	}

	// nolint:forbidigo
	fmt.Printf("port: %d\n", port)

	// TODO retry logic to handle weird port things
	// wait 100ms for port to be ready with retry backup
	conn := fmt.Sprintf("%s:%d", "0.0.0.0", port)

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
