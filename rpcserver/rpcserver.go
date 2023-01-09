package rpcserver

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"net/rpc"
	"os"
	"validation-bot/module"
	"validation-bot/task"

	cborutil "github.com/filecoin-project/go-cbor-util"
	"github.com/pkg/errors"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
)

type RPCServer struct {
	log     zerolog.Logger
	Modules map[task.Type]module.AuditorModule
}

type Config struct {
	Modules map[task.Type]module.AuditorModule
}

func NewRPCServer(config Config) *RPCServer {
	return &RPCServer{
		log:     log.With().Str("role", "rpcserver").Caller().Logger(),
		Modules: config.Modules,
	}
}

func (ra *RPCServer) Validate(input module.ValidationInput, reply *module.ValidationResult) error {
	ctx := context.Background()
	ra.log.Info().Msgf("Received validation request for task %s", input.TaskID)

	mod, ok := ra.Modules[input.Type]
	if !ok {
		return errors.New(fmt.Sprintf("no module found for task type %s", input.Type))
	}

	result, err := mod.Validate(ctx, input)
	if err != nil {
		//nolint:forbidigo
		fmt.Printf("Error validating task %s: %s", input.Type, err)
		return errors.Wrap(err, "failed validating task")
	}

	*reply = *result
	return nil
}

type portNumber = int

func (ra *RPCServer) Start(ctx context.Context, forcePort int) error {
	rpcServer := new(RPCServer)
	rpcServer.Modules = ra.Modules

	err := rpc.Register(rpcServer)
	if err != nil {
		return errors.Wrap(err, "failed to register rpc auditor")
	}

	rpc.HandleHTTP()

	var address string
	if forcePort != 0 {
		address = fmt.Sprintf("0.0.0.0:%d", forcePort)
	} else {
		address = "0.0.0.0:"
	}

	listener, _ := net.Listen("tcp", address)
	addr, ok := listener.Addr().(*net.TCPAddr)
	if !ok {
		return errors.New("failed type assertion on listener.Addr to *net.TCPAddr")
	}

	if err := cborutil.WriteCborRPC(os.Stdout, addr.Port); err != nil {
		return fmt.Errorf("failed to send request: %w", err)
	}

	// print port number to stdout so ClientRPC can read it
	//nolint:forbidigo
	// fmt.Printf("%d\n", addr.Port)

	done := make(chan struct{})
	defer close(done)

	go func() {
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
		log.Info().Msgf("shutting down Validator RPC Server on port: %q", address)
		return nil
	default:
		if err != nil {
			return errors.Wrap(err, "failed to accept connection")
		}

		// nolint:gosec
		// http.Serve always returns non-nil error when closing: ignore
		_ = http.Serve(listener, nil)

		return nil
	}
}
