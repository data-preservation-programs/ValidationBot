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
}

type ClientConfig struct {
	BaseDir  string
	Timeout  time.Duration
	ExecPath string
}

const (
	scanPause = time.Millisecond * 200
	scanLoops = 3
)

func NewClientRPC(config ClientConfig) *ClientRPC {
	return &ClientRPC{
		log:      log.With().Str("role", "rpc.client").Caller().Logger(),
		baseDir:  config.BaseDir,
		Timeout:  config.Timeout,
		execPath: config.ExecPath,
	}
}

func (r *ClientRPC) GetTimeout() time.Duration {
	return r.Timeout
}

func (r *ClientRPC) CallServer(
	ctx context.Context,
	input module.ValidationInput,
) (*module.ValidationResult, error) {
	dir, err := os.MkdirTemp(r.baseDir, "validation_rpc")
	if err != nil {
		return nil, errors.Wrap(err, "failed to create directory")
	}

	defer func() {
		r.log.Info().Str("defered removal of dir", dir).Msg("removing tmp dir!")
		os.RemoveAll(dir)
	}()

	absdir, err := filepath.Abs(dir)
	if err != nil {
		return nil, errors.Wrap(err, "failed to get absolute path")
	}

	r.log.Info().Str("Absolute dir", absdir).Msg("using absolute path to tmp dir")
	r.log.Info().Str("Exec Path", r.execPath).Msg("executing validation bot rpc from path")

	// calls /path/to/ValidationBot/validation_bot validation-rpc
	cmd := exec.CommandContext(ctx, dir, "validation-rpc")
	cmd.Dir = absdir

	cmd.Path = fmt.Sprintf("%s/validation_bot", r.execPath)

	r.log.Info().Str("cmd.Path", cmd.Path).Msg("executing validation bot rpc from cmd.Path")

	stdout, _ := cmd.StdoutPipe()
	defer stdout.Close()

	err = cmd.Start()
	if err != nil {
		return nil, errors.Wrap(err, "failed to start validation server")
	}

	defer func() {
		if rec := recover(); rec != nil {
			log.Error().Err(errors.Errorf("%v", rec)).Msg("panic - killing process")

			err = cmd.Process.Kill()
			if err != nil {
				log.Error().Err(err).Msg("failed to kill process")
			}
		}
	}()

	reply, err := r.Validate(ctx, stdout, input)
	if err != nil {
		return nil, errors.Wrap(err, "failed to call validate")
	}

	select {
	case <-ctx.Done():
		err = cmd.Process.Kill()
		if err != nil {
			return nil, errors.Wrap(err, "failed to kill process")
		}
		return nil, errors.New("context cancelled")
	default:
		err = cmd.Process.Kill()
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
	// listen for rpc server port from stdout - fmt.Printf("%d\n", addr.Port)
	scanner := bufio.NewScanner(stdout)

	var port int
	scans := 0

	for scanner.Scan() {
		if _port, err := strconv.Atoi(scanner.Text()); err != nil {
			scans += 1
			if scans > scanLoops {
				return nil, errors.Wrap(err, "failed to parse port")
			}
			time.Sleep(scanPause)
		} else {
			port = _port
			break
		}
	}

	// nolint:forbidigo
	log.Info().Msgf("port detected: %d\n", port)
	conn := fmt.Sprintf("%s:%d", "0.0.0.0", port)

	log.Info().Msgf("dialing http: %s\n", conn)
	client, err := rpc.DialHTTP("tcp", conn)
	if err != nil {
		return nil, errors.Wrap(err, "failed to dial http")
	}

	defer client.Close()

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

	var reply module.ValidationResult

	log.Info().Msg("calling rpc validate")
	err = client.Call("RPCServer.Validate", input, &reply)
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
