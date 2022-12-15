package rpc

import (
	ret "validation-bot/module/retrieval"
)

type RpcClient struct {
	GraphSync *ret.GraphSyncRetriever
	Http      struct{}
	BitSwap   struct{}
}

func NewRpcClient() *RpcClient {
	return &RpcClient{}
}

func (r *RpcClient) Run(message string) (string, error) {
	return r.retrieval.Send(message)
}
