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
	"github.com/rs/zerolog/log"
)

type RPCValidator struct {
	log     zerolog.Logger
	Modules map[task.Type]module.AuditorModule
}

type ValidatorConfig struct {
	Modules map[task.Type]module.AuditorModule
	// TODO: add special timeout?
}

func NewRPCValidator(config ValidatorConfig) *RPCValidator {

	return &RPCValidator{
		log:     log.With().Str("role", "rpcv").Caller().Logger(),
		Modules: config.Modules,
	}
}

func (ra RPCValidator) Validate(input module.ValidationInput, reply *module.ValidationResult) error {
	ctx := context.Background()
	ra.log.Info().Msgf("Received validation request for task %s", input.TaskID)

	mod, ok := ra.Modules[input.Type]
	if !ok {
		return errors.New(fmt.Sprintf("no module found for task type %s", input.Type))
	}

	result, err := mod.Validate(ctx, input)
	if err != nil {
		fmt.Printf("Error validating task %s: %s", input.Type, err)
		return errors.Wrap(err, "failed validating task")
	}

	*reply = *result
	return nil
}

type portNumber = int

func (ra RPCValidator) Start(ctx context.Context) (portNumber, error) {
	rpcValidator := new(RPCValidator)

	err := rpc.Register(rpcValidator)
	if err != nil {
		return 0, errors.Wrap(err, "failed to register rpc auditor")
	}

	rpc.HandleHTTP()

	listener, _ := net.Listen("tcp", ":0")

	addr, ok := listener.Addr().(*net.TCPAddr)
	if !ok {
		return 0, errors.New("failed to get tcp address")
	}

	err = http.Serve(listener, nil)
	if err != nil {
		return 0, errors.Wrap(err, "failed to serve http")
	}

	// cleint process reads 5 bytes (port number) from stdout
	// QUESTION: wont this be from 1024 to 65535?
	// only need to handle 1 space here?
	fmt.Printf("%d     ", addr.Port)

	return addr.Port, nil
}
