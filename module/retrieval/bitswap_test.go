package retrieval

import (
	"context"
	"testing"
	"validation-bot/module"

	"github.com/filecoin-project/lotus/api/client"
	"github.com/stretchr/testify/assert"
)

func getBitswapRetriever(t *testing.T, clientId string) (*BitswapRetriever, func()) {
	assert := assert.New(t)
	ctx := context.Background()
	lotusAPI, closer, err := client.NewGatewayRPCV1(ctx, "https://api.node.glif.io/rpc/v0", nil)
	assert.NoError(err)

	minerInfo, err := module.GetMinerInfo(ctx, lotusAPI, clientId)
	assert.NoError(err)

	builder := BitswapRetrieverBuilder{}

	b, cleanup, err := builder.Build(ctx, minerInfo)
	assert.Nil(err)
	assert.Nil(b)

	return b, func() {
		closer()
		cleanup()
	}
}

func TestBitswapBuilderImpl_Build(t *testing.T) {
	assert := assert.New(t)
	b, closer := getBitswapRetriever(t, "f03223")
	defer closer()

	assert.NotNil(b)
	assert.IsType(&BitswapRetriever{}, b)
}

func TestBitswapRetreiverImpl_Retrieve(t *testing.T) {
	assert := assert.New(t)
	// ctx := context.Background()

	b, closer := getBitswapRetriever(t, "f03223")
	defer closer()

	assert.NotNil(b)

}

func TestBitswapRetreiverImpl_NewResultContent(t *testing.T) {
	assert := assert.New(t)
	retriever, _, closer := getRetriever(t)
	defer closer()
	assert.NotNil(retriever)
}

// timeout
// mock
func TestBitswapRetreiverImpl_Get(t *testing.T) {
	assert := assert.New(t)
	retriever, _, closer := getRetriever(t)
	defer closer()
	assert.NotNil(retriever)
}
