package rpcv

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"net/rpc"
	"time"
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

func (ra RPCValidator) Start(ctx context.Context, forcePort int) error {
	// TODO: why not pass ra into Register?
	rpcValidator := new(RPCValidator)

	err := rpc.Register(rpcValidator)
	if err != nil {
		return errors.Wrap(err, "failed to register rpc auditor")
	}

	rpc.HandleHTTP()

	listener, _ := net.Listen("tcp", ":0")

	addr := listener.Addr().(*net.TCPAddr)

	var port portNumber
	if forcePort == 0 {
		port = portNumber(addr.Port)
	} else {
		// for testing
		port = portNumber(forcePort)
	}
	// addr, ok := listener.Addr().(*net.TCPAddr)

	fmt.Print("testing")
	// cleint process reads 5 bytes (port number) from stdout
	// QUESTION: wont this be from 1024 to 65535?
	// only need to handle 1 space here?
	// http.Serve will block until the listener is closed
	// str := fmt.Sprintf("%q", addr.Port)
	// TODO visit this later
	go func() {
		time.Sleep(100 * time.Millisecond)
		fmt.Printf("%d\n", port)
	}()

	// print port number to stdout
	fmt.Printf("%d     ", port)

	err = http.Serve(listener, nil)
	if err != nil {
		return errors.Wrap(err, "failed to serve http")
	}

	<-ctx.Done()
	log.Info().Msgf("shutting down Validator RPC on port: %q", port)
	return nil
}
