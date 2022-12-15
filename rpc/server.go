package rpc

import (
	"errors"
	"fmt"
	"net"
	"net/http"
	"net/rpc"
)

type Args struct {
	A, B int
}

type Quotient struct {
	Quo, Rem int
}

type Arith int

func (t *Arith) Multiply(args *Args, reply *int) error {
	*reply = args.A * args.B
	return nil
}

func (t *Arith) Divide(args *Args, quo *Quotient) error {
	if args.B == 0 {
		return errors.New("divide by zero")
	}
	quo.Quo = args.A / args.B
	quo.Rem = args.A % args.B
	return nil
}

func NewRpcServer() (*net.TCPAddr, error) {
	arith := new(Arith)
	rpc.Register(arith)
	rpc.HandleHTTP()

	listener, _ := net.Listen("tcp", ":0")

	fmt.Println("Using port:", listener.Addr().(*net.TCPAddr).Port)

	http.Serve(listener, nil)

	return listener.Addr().(*net.TCPAddr), nil
}
