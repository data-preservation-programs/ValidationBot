package rpcv

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestRpcServer_NewRPCServer(t *testing.T) {
	assert := assert.New(t)
	server, err := NewRPCServer()

	assert.NotNil(server.Port)
	assert.NotNil(server.IP)
	assert.Nil(err)
}
