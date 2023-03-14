package auditor

import (
	"context"
	"fmt"
	"io/ioutil"
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
	CallServer(ctx context.Context, dir string, input module.ValidationInput) (*module.ValidationResult, error)
	Validate(
		ctx context.Context,
		port int,
		input module.ValidationInput,
	) (*module.ValidationResult, error)
	GetTimeout() time.Duration
	CreateTmpDir() (string, error)
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
	retryPause = time.Millisecond * 200
	retryMax   = 10
	tmpDir     = "validation_rpc"
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

func (r *ClientRPC) CreateTmpDir() (string, error) {
	dir, err := os.MkdirTemp(r.baseDir, tmpDir)
	if err != nil {
		return "", errors.Wrap(err, "failed to create directory")
	}

	return dir, nil
}

func (r *ClientRPC) CallServer(
	ctx context.Context,
	dir string,
	input module.ValidationInput,
) (*module.ValidationResult, error) {
	absdir, err := filepath.Abs(dir)
	if err != nil {
		r.log.Error().Err(err).Msg("failed to get absolute path")
		return nil, errors.Wrap(err, "failed to get absolute path")
	}

	r.log.Info().Str("Absolute dir", absdir).Msg("using absolute path to tmp dir")
	r.log.Info().Str("Exec Path", r.execPath).Msg("executing validation bot rpc from path")

	// calls /path/to/ValidationBot/validation_bot validation-rpc with tmp dir as arg
	// the rpcserver will write the port to a file in the tmp dir so it can be read
	// in the Validate call
	cmd := exec.CommandContext(ctx, dir, "validation-rpc", "-dir", absdir)
	cmd.Dir = absdir

	cmd.Path = fmt.Sprintf("%s/validation_bot", r.execPath)

	r.log.Info().Str("cmd.Path", cmd.Path).Msg("executing validation bot rpc from cmd.Path")

	err = cmd.Start()
	if err != nil {
		r.log.Error().Err(err).Msg("failed to start validation server")
		return nil, errors.Wrap(err, "failed to start validation server")
	}

	var port int
	readCount := 0

	for {
		//nolint:varnamelen
		p, err := ioutil.ReadFile(fmt.Sprintf("%s/%s", absdir, "port.txt"))
		readCount += 1
		if err != nil {
			if readCount >= retryMax {
				os.RemoveAll(dir)
				r.log.Error().Err(err).Msg("retry count exhausted, failed to read port.txt")
				return nil, errors.Wrap(err, "retry count exhausted, failed to read port.txt")
			}

			r.log.Info().Msgf("retry count: %d; retry max: %d; retrying in %d seconds...", readCount, retryMax, retryPause)
			time.Sleep(retryPause)
			continue
		}

		r.log.Info().Msgf("port.txt returned: %s\n", p)
		_port, err := strconv.Atoi(string(p))
		if err != nil {
			os.RemoveAll(dir)
			return nil, errors.Wrap(err, "failed to parse port from port.txt")
		}

		port = _port
		break
	}

	defer func() {
		r.log.Info().Str("defered removal of dir", dir).Msg("removing tmp dir!")
		os.RemoveAll(dir)

		if rec := recover(); rec != nil {
			log.Error().Err(errors.Errorf("%v", rec)).Msg("panic - killing process")

			err = cmd.Process.Kill()
			if err != nil {
				log.Error().Err(err).Msg("failed to kill process")
			}
		}
	}()

	reply, err := r.Validate(ctx, port, input)
	if err != nil {
		r.log.Error().Err(err).Msg("failed to call validate")
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
	port int,
	input module.ValidationInput,
) (*module.ValidationResult, error) {
	// nolint:forbidigo
	log.Info().Msgf("port detected: %d\n", port)
	conn := fmt.Sprintf("%s:%d", "0.0.0.0", port)

	log.Info().Msgf("dialing http: %s\n", conn)
	client, err := rpc.DialHTTP("tcp", conn)
	log.Info().Msgf("client: %v\n", client)
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

	// if RPCServer.Validate is not working, you can check the
	// rpc connection with:
	// var pong string
	// err = client.Call("RPCServer.Ping", "ping", &pong)
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
