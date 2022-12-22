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
}

func NewRPCValidator(config ValidatorConfig) *RPCValidator {

	return &RPCValidator{
		log:     log.With().Str("role", "rpcv").Caller().Logger(),
		Modules: config.Modules,
	}
}

func (ra *RPCValidator) Validate(input module.ValidationInput, reply *module.ValidationResult) error {
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

func (ra *RPCValidator) Start(ctx context.Context, forcePort int) error {
	rpcValidator := new(RPCValidator)
	rpcValidator.Modules = ra.Modules

	err := rpc.Register(rpcValidator)
	if err != nil {
		return errors.Wrap(err, "failed to register rpc auditor")
	}

	rpc.HandleHTTP()

	// fmt.Print(addr.String())
	var address string
	if forcePort != 0 {
		// for testing
		address = fmt.Sprintf("0.0.0.0:%d", forcePort)
	} else {
		address = "0.0.0.0:"
	}

	listener, _ := net.Listen("tcp", address)
	addr := listener.Addr().(*net.TCPAddr)

	// cleint process reads 5 bytes (port number) from stdout
	// QUESTION: wont this be from 1024 to 65535?
	// only need to handle 1 space here?
	// print port number to stdout
	// fmt.Printf("%d     ", port)
	fmt.Printf("%d\n", addr.Port)

	done := make(chan struct{})
	// ensure listener is closed
	defer close(done)

	go func() {
		// conn, err := listener.Accept()
		// close connection
		// defer conn.Close()
		// rpc.DefaultServer.Accept(listener)

		if err != nil {
			log.Error().Err(err).Msg("failed to accept connection")
			return
		}

		select {
		case <-done:
			listener.Close()
		case <-ctx.Done():
			listener.Close()
		}
	}()

	select {
	case <-ctx.Done():
		log.Info().Msgf("shutting down Validator RPC on port: %q", address)
		return nil
	default:
		// err = errors.New(fmt.Sprintf("Cannot start validator RPC on port: %q", addr.Port))
		// conn, err :=
		// fmt.Println("conn: ", conn)

		if err != nil {
			return errors.Wrap(err, "failed to accept connection")
		}

		http.Serve(listener, nil)
		// if err != nil {
		// ignore error and close? Serve always returns non nil error
		// return errors.Wrap(err, "failed to serve http")
		// }
		return nil
	}
}
