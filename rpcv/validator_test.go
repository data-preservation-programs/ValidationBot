package rpcv

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestRpcServer_NewRPCServer(t *testing.T) {
	assert := assert.New(t)
	port, err := NewRPCValidator()

	assert.IsType(0, port)
	assert.Nil(err)
	// implement client call so server closes
}
