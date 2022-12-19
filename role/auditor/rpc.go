package auditor

import (
	"context"
	"fmt"
	"io"
	"net/rpc"
	"os"
	"os/exec"
	"path"
	"strconv"
	"strings"
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
}

func NewRPCClient(baseDir string) *RPCClient {
	return &RPCClient{
		log:     log.With().Str("role", "rpc.client").Caller().Logger(),
		baseDir: baseDir,
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

	defer os.RemoveAll(dirPath)

	cmd := exec.CommandContext(ctx, "validation-rpc", dirPath)
	stdout, _ := cmd.StdoutPipe()

	defer stdout.Close()

	err = cmd.Start()
	if err != nil {
		return nil, errors.Wrap(err, "failed to start validation server")
	}

	buf := make([]byte, 5)

	_, err = io.ReadAtLeast(stdout, buf, 5)
	if err != nil {
		return nil, errors.Wrap(err, "failed to read from stdout")
	}

	port, err := strconv.Atoi(strings.TrimSpace(string(buf)))
	if err != nil {
		return nil, errors.Wrap(err, "failed to parse port")
	}

	client, err := rpc.DialHTTP("tcp", fmt.Sprintf("%s:%d", "localhost", port))
	if err != nil {
		return nil, errors.Wrap(err, "failed to dial http")
	}

	defer client.Close()

	var reply module.ValidationResult

	valid := client.Go("RPCAuditor.Validate", input, &reply, nil)

	select {
	case <-ctx.Done():
		cmd.Process.Kill()
		return nil, errors.New("context cancelled")
	case <-valid.Done:
		if valid.Error != nil {
			return nil, errors.Wrap(valid.Error, "failed to validate")
		}

		cmd.Process.Kill()
		return &reply, nil
	}
}
