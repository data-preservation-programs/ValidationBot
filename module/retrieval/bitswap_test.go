package retrieval

import (
	"context"
	"fmt"
	"testing"
	"time"
	"validation-bot/module"

	"github.com/filecoin-project/lotus/api/client"
	cid "github.com/ipfs/go-cid"
	ipld "github.com/ipfs/go-ipld-format"
	"github.com/ipfs/go-merkledag"
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

func TestonNewBlockImpl(t *testing.T) {
	assert := assert.New(t)
	b, closer := getBitswapRetriever(t, "f03223")
	defer closer()

	c := cid.NewCidV1(cid.Raw, []byte("hello world"))
	block := Block{
		BlockCID: c,
		Data:     c.Bytes(),
		Offset:   0,
		Size:     uint64(len(c.Bytes())),
	}

	b.onNewBlock(block)

	assert.Equal(len(b.events), 1)
	assert.Equal(b.events[0].Code, string(FirstByteReceived))

	b.onNewBlock(block)

	assert.Equal(len(b.events), 2)
	assert.Equal(b.events[1].Code, string(BlockReceived))
}

func TestBitswapGetImpl(t *testing.T) {
	assert := assert.New(t)

	b, closer := getBitswapRetriever(t, "f03223")
	defer closer()

	v := []byte("hello world")
	c := cid.NewCidV1(cid.Raw, v)

	t.Run("Get() returns Block with duration logged", func(t *testing.T) {
		rs := new(mockReadStore)
		rs.On("Get", mock.Anything, c).Return(v, nil)
		b.bitswap = rs

		ctx := context.Background()

		result, err := b.Get(ctx, c)
		assert.NoError(err)

		assert.Equal(v, result)
		assert.Equal(len(b.cidDurations), 1)
	})

	t.Run("Get() returns an error and records the event", func(t *testing.T) {
		rs := new(mockReadStore)
		rs.On("Get", mock.Anything, c).Return(v, errors.New("error"))
		b.bitswap = rs

		ctx := context.Background()

		_, err := b.Get(ctx, c)
		assert.Error(err, "error")
		assert.Equal(len(b.cidDurations), 1)
		assert.Equal(len(b.events), 1)
	})
}

func makeDepthTestingGraph(t *testing.T) (ipld.Node, map[cid.Cid]ipld.Node) {
	root := merkledag.NodeWithData(nil)
	l11 := merkledag.NodeWithData([]byte("leve1_node1"))
	l12 := merkledag.NodeWithData([]byte("leve1_node2"))
	l21 := merkledag.NodeWithData([]byte("leve2_node1"))
	l22 := merkledag.NodeWithData([]byte("leve2_node2"))
	l23 := merkledag.NodeWithData([]byte("leve2_node3"))

	l11.AddNodeLink(l21.Cid().String(), l21)
	l11.AddNodeLink(l22.Cid().String(), l22)
	l11.AddNodeLink(l23.Cid().String(), l23)

	root.AddNodeLink(l11.Cid().String(), l11)
	root.AddNodeLink(l12.Cid().String(), l12)
	root.AddNodeLink(l23.Cid().String(), l23)

	var nodeMap = map[cid.Cid]ipld.Node{}

	for _, n := range []ipld.Node{l23, l22, l21, l12, l11, root} {
		nodeMap[n.Cid()] = n
	}

	return root, nodeMap
}

func TestRetreiveImpl(t *testing.T) {
	assert := assert.New(t)

	bit, closer := getBitswapRetriever(t, "f03223")
	defer closer()

	rs := new(mockReadStore)
	root, nodeMap := makeDepthTestingGraph(t)

	rs.On("Close").Return(nil)
	for k, v := range nodeMap {
		fmt.Printf("mocking from nodeMap: key: %s, value: %v\n\n", k, v)
		rs.On("Get", mock.Anything, k).Return(v.RawData(), nil)
	}

	bit.bitswap = rs

	t.Run("Retreive() hits deadline", func(t *testing.T) {
		ctx := context.Background()

		result, err := bit.Retrieve(ctx, root.Cid(), 8*time.Second)

		for i, s := range result.CalculatedStats.Events {
			fmt.Printf("stat-%d: %v\n", i, s)
		}
		for i, e := range bit.events {
			fmt.Printf("event-%d: %v\n", i, e)
		}

		fmt.Printf("AverageSpeedPerSec: %v\n", result.AverageSpeedPerSec)
		fmt.Printf("BytesDownloaded: %v\n", result.BytesDownloaded)
		fmt.Printf("TimeElapsed: %v\n", result.TimeElapsed)
		fmt.Printf("numberOfEvents: %v\n", len(result.Events))

		assert.Nil(err)
		assert.GreaterOrEqual(len(bit.cidDurations), 1)
		assert.GreaterOrEqual(len(bit.events), 1)
		assert.Equal(result.Status, Success)
		assert.Greater(result.TimeElapsed, time.Duration(0))
		assert.NotNil(result.BytesDownloaded)
		assert.NotNil(result.TimeToFirstByte)
		assert.NotNil(result.AverageSpeedPerSec)
	})
}

func TestBitswapGetImplLive(t *testing.T) {
	// t.Skip("Only turn on for live test")
	assert := assert.New(t)

	b, closer := getBitswapRetriever(t, "f022352")
	defer closer()

	c, err := cid.Decode("bafybeicozcxftee3qct6mswo7nmchbmyhstqbsteytbdiarq64kqcooa34")
	assert.NoError(err)

	t.Run("Get() returns Block with duration logged", func(t *testing.T) {
		result, err := b.Retrieve(context.Background(), c, 8*time.Minute)
		assert.NoError(err)

		fmt.Printf("result: %v\n", result)
		assert.NotNil(result)
		assert.Equal(len(b.cidDurations), 1)
		assert.Greater(result.BytesDownloaded, uint64(0))
	})
}
