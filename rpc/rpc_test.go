package rpc

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestRpcClient_Send(t *testing.T) {
	assert := assert.New(t)
	client := RpcClient{}
	_, err := client.Run("helloworld")
	assert.Nil(err)
}
