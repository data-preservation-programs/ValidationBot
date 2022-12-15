package rpc

import (
	"fmt"
	"net/rpc"
	"os"

	"github.com/pkg/errors"
)

type GraphSyncOptions struct {
	Args []string
}

type HttpOptions struct {
	Args []string
}

type BitSwapOptions struct {
	Args []string
}

type CommandArgs struct {
	ArithArgs []string
	GraphSync Args
	Http      Args
	BitSwap   Args
}

func main() {
	// TODO validate os.Args for server/client

	// Start Server
	addr, err := NewRpcServer()

	if err != nil {
		errors.Wrap(err, "failed to start RPC server")
	}

	fmt.Println("RPC server started on %s:%s", addr.IP, addr.Port)

	if os.Args[0] == "client" && addr.Port != 0 {
		client, err := rpc.DialHTTP("tcp", fmt.Sprintf("%s:%d", addr.IP, addr.Port))

		if err != nil {
			errors.Wrap(err, "failed to dial RPC server")
		}

		// Synchronous call
		args := Args{17, 8}

		var reply int

		err = client.Call("Arith.Multiply", args, &reply)

		if err != nil {
			errors.Wrap(err, "failed to call RPC server")
		}

		fmt.Printf("Arith: %d*%d=%d", args.A, args.B, reply)
	}
}
