package rpcv

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

// TODO configure this
type RPCValidator struct {
	log     zerolog.Logger
	modules map[task.Type]module.AuditorModule
}

func (ra RPCValidator) Validate(ctx context.Context, input module.ValidationInput) (*module.ValidationResult, error) {
	ra.log.Info().Msgf("Received validation request for task %s", input.TaskID)

	mod, ok := ra.modules[input.Type]
	if !ok {
		return nil, errors.New(fmt.Sprintf("no module found for task type %s", input.Type))
	}

	result, err := mod.Validate(ctx, input)
	if err != nil {
		fmt.Printf("Error validating task %s: %s", input.Type, err)
		return nil, errors.Wrap(err, "failed validating task")
	}

	return result, nil
}

func NewRPCServer() (*net.TCPAddr, error) {
	rpcValidator := new(RPCValidator)

	err := rpc.Register(rpcValidator)
	if err != nil {
		return nil, errors.Wrap(err, "failed to register rpc auditor")
	}

	rpc.HandleHTTP()

	listener, _ := net.Listen("tcp", ":0")

	addr, ok := listener.Addr().(*net.TCPAddr)
	if !ok {
		return nil, errors.New("failed to get tcp address")
	}

	fmt.Println("Using port:", addr.Port)

	err = http.Serve(listener, nil)
	if err != nil {
		return nil, errors.Wrap(err, "failed to serve http")
	}

	return listener.Addr().(*net.TCPAddr), nil
}
