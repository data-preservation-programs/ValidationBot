package rpcserver

import (
	"context"
	"fmt"
	"io/ioutil"
	"net"
	"net/http"
	"net/rpc"
	"os"
	"strconv"
	"validation-bot/module"
	"validation-bot/task"

	"github.com/pkg/errors"
	"github.com/rs/zerolog"
)

type RPCServer struct {
	log     zerolog.Logger
	Modules map[task.Type]module.AuditorModule
}

type Config struct {
	Modules map[task.Type]module.AuditorModule
}

func NewRPCServer(config Config) *RPCServer {
	// Create a ConsoleWriter output writer that writes to standard output
	consoleWriter := zerolog.ConsoleWriter{Out: os.Stdout}

	// Create a new logger with the ConsoleWriter output writer
	log := zerolog.New(consoleWriter).With().Str("role", "rpc-server").Caller().Timestamp().Logger()

	return &RPCServer{
		log:     log,
		Modules: config.Modules,
	}
}

func (ra *RPCServer) Ping(input string, reply *string) error {
	ra.log.Info().Msg("Received ping")
	*reply = "pong"
	return nil
}

func (ra *RPCServer) Validate(input module.ValidationInput, reply *module.ValidationResult) error {
	ctx := context.Background()
	ra.log.Info().Msgf("Received validation request for task %s", input.TaskID)

	mod, ok := ra.Modules[input.Type]
	if !ok {
		ra.log.Error().Msgf("No module found for task type %s", input.Type)
		return errors.New(fmt.Sprintf("no module found for task type %s", input.Type))
	}

	result, err := mod.Validate(ctx, input)
	if err != nil {
		//nolint:forbidigo
		ra.log.Error().Msgf("Error validating task %s: %s", input.Type, err)
		return errors.Wrap(err, "failed validating task")
	}

	ra.log.Info().Msgf("Validation result for task %s: %v", input.Type, result)

	*reply = *result
	return nil
}

type portNumber = int

func (ra *RPCServer) Start(ctx context.Context, forcePort portNumber, tmpDir string) error {
	rpcServer := new(RPCServer)
	rpcServer.Modules = ra.Modules

	err := rpc.Register(rpcServer)
	if err != nil {
		ra.log.Error().Err(err).Msg("failed to register rpc auditor")
		return errors.Wrap(err, "failed to register rpc auditor")
	}

	rpc.HandleHTTP()

	var address string
	if forcePort != 0 {
		address = fmt.Sprintf("0.0.0.0:%d", forcePort)
	} else {
		address = ":0"
	}

	listener, _ := net.Listen("tcp", address)
	ra.log.Info().Msgf("listening on %s", listener.Addr().String())

	addr, ok := listener.Addr().(*net.TCPAddr)
	if !ok {
		ra.log.Error().Msg("failed type assertion on listener.Addr to *net.TCPAddr")
		return errors.New("failed type assertion on listener.Addr to *net.TCPAddr")
	}

	ra.log.Info().Msgf("Validator RPC Server listening on port: %d", addr.Port)

	//nolint:forbidigo
	// write port number to tmpdir/port.txt received from client side so ClientRPC can read it
	_port := []byte(strconv.Itoa(addr.Port))
	ra.log.Info().Msgf("writing to %s to %s/port.txt", _port, tmpDir)

	//nolint:gomnd
	err = ioutil.WriteFile(fmt.Sprintf("%s/port.txt", tmpDir), []byte(strconv.Itoa(addr.Port)), 0600)
	if err != nil {
		ra.log.Error().Err(err).Msg("failed to write port to file")
		return errors.Wrap(err, "failed to write port to file")
	}

	done := make(chan struct{})
	defer close(done)

	// ensure the listener closes
	go func() {
		if err != nil {
			ra.log.Error().Err(err).Msg("failed to accept connection on rpcserver")
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
		ra.log.Info().Msgf("shutting down Validator RPC Server on port: %q", address)
		return nil
	default:
		if err != nil {
			ra.log.Error().Err(err).Msg("failed to accept connection on rpcserver")
			return errors.Wrap(err, "failed to accept connection")
		}

		// nolint:gosec
		// http.Serve always returns non-nil error when closing: ignore
		ra.log.Info().Msg("starting rpc server")
		_ = http.Serve(listener, nil)
		ra.log.Info().Msg("shutting down rpc server")
		return nil
	}
}
