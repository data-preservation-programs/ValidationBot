package validation_server

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"net/rpc"
	"validation-bot/module"
	"validation-bot/task"

	"github.com/pkg/errors"

	"github.com/rs/zerolog"
)

type RpcAuditor struct {
	log     zerolog.Logger
	modules map[task.Type]module.AuditorModule
}

func (ra RpcAuditor) Validate(ctx context.Context, input module.ValidationInput) (*module.ValidationResult, error) {
	ra.log.Info().Msgf("Received validation request for task %s", input.TaskID)

	modType := input.Type

	mod, ok := ra.modules[modType]
	if !ok {
		return nil, errors.New(fmt.Sprintf("no module found for task type %s", input.Type))
	}

	result, err := mod.Validate(ctx, input)
	if err != nil {
		fmt.Printf("Error validating task %s: %s", input.TaskID, err)
		return nil, errors.Wrap(err, "failed validating task")
	}

	return result, nil
}

func NewRpcServer() (*net.TCPAddr, error) {
	rpcAuditor := new(RpcAuditor)

	rpc.Register(rpcAuditor)
	rpc.HandleHTTP()

	listener, _ := net.Listen("tcp", ":0")

	fmt.Println("Using port:", listener.Addr().(*net.TCPAddr).Port)

	http.Serve(listener, nil)

	return listener.Addr().(*net.TCPAddr), nil
}
