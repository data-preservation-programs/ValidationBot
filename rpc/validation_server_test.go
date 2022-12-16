package validation_server

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestRpcServer_NewRpcServer(t *testing.T) {
	assert := assert.New(t)
	server, err := NewRpcServer()

	assert.NotNil(server.Port)
	assert.NotNil(server.IP)
	assert.Nil(err)
}
