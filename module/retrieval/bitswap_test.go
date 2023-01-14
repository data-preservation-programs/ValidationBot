package retrieval

import (
	"context"
	"testing"
	"validation-bot/module"

	"github.com/filecoin-project/lotus/api/client"
	blocks "github.com/ipfs/go-block-format"
	cid "github.com/ipfs/go-cid"
	gocar "github.com/ipld/go-car"
	"github.com/pkg/errors"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
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
	assert.NotNil(b)

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

func TestBitswapRetreiverImpl_RetrieveGet(t *testing.T) {
	assert := assert.New(t)

	b, closer := getBitswapRetriever(t, "f03223")
	defer closer()

	c := cid.NewCidV1(cid.Raw, []byte("hello world"))

	block := blocks.NewBlock([]byte("hello world"))

	t.Run("Get() returns Block with duration logged", func(t *testing.T) {
		rs := new(mockReadStore)
		rs.On("Get", mock.Anything, mock.Anything).Return(block, nil)
		b.bitswap = func() gocar.ReadStore { return rs }

		ctx := context.Background()

		result, err := b.Get(ctx, c)
		assert.NoError(err)

		assert.Equal(block.RawData(), result.RawData())
		assert.Equal(len(b.cidDurations), 1)
	})

	t.Run("Get() returns an error and records the event", func(t *testing.T) {
		rs := new(mockReadStore)
		rs.On("Get", mock.Anything, mock.Anything).Return(block, errors.New("error"))
		b.bitswap = func() gocar.ReadStore { return rs }

		ctx := context.Background()

		_, err := b.Get(ctx, c)
		assert.Error(err, "error")
		assert.Equal(len(b.cidDurations), 1)
		assert.Equal(len(b.events), 1)
	})
}

// TODO: add Retreive test cases
