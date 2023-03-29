package retrieval

import (
	"context"
	"fmt"
	"testing"
	"time"
	"validation-bot/module"
	"validation-bot/role"

	"github.com/filecoin-project/boost/retrievalmarket/types"
	"github.com/filecoin-project/lotus/api/client"
	cid "github.com/ipfs/go-cid"
	ipld "github.com/ipfs/go-ipld-format"
	"github.com/ipfs/go-libipfs/blocks"
	"github.com/ipfs/go-merkledag"
	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/pkg/errors"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

func getBitswapRetriever(t *testing.T, clientId string, getProtos bool) (*BitswapRetriever, func()) {
	assert := assert.New(t)
	ctx := context.Background()
	lotusAPI, closer, err := client.NewGatewayRPCV1(ctx, "https://api.node.glif.io/rpc/v0", nil)
	assert.NoError(err)

	minerInfo, err := module.GetMinerInfo(context.Background(), lotusAPI, clientId)
	assert.NoError(err)

	libp2p, err := role.NewLibp2pHostWithRandomIdentityAndPort()
	if err != nil {
		panic(err)
	}

	var bitswap MinerProtocols

	if getProtos {
		protoprovider := NewProtocolProvider(libp2p)

		// For Live Testing
		protocols, err := protoprovider.GetMinerProtocols(ctx, peer.AddrInfo{ID: *minerInfo.PeerID, Addrs: minerInfo.MultiAddrs})
		if err != nil {
			panic(err)
		}

		for _, mp := range protocols {
			if mp.Protocol.Name == "bitswap" {
				bitswap = mp
			}
		}

		fmt.Printf("bitswap: %v\n", bitswap)
	} else {
		bitswap = MinerProtocols{Protocol: types.Protocol{Name: string(Bitswap)}}
	}

	builder := BitswapRetrieverBuilder{}
	fmt.Println("minerInfo: ", minerInfo)

	b, cleanup, err := builder.Build(minerInfo, bitswap, libp2p)
	assert.Nil(err)
	assert.NotNil(b)

	return b, func() {
		closer()
		cleanup()
	}
}

func TestBitswapBuilderImpl_Build(t *testing.T) {
	assert := assert.New(t)
	b, closer := getBitswapRetriever(t, "f03223", false)
	defer closer()

	assert.NotNil(b)
	assert.IsType(&BitswapRetriever{}, b)
}

func TestBitswapGetImpl(t *testing.T) {
	assert := assert.New(t)

	b, closer := getBitswapRetriever(t, "f03223", false)
	defer closer()

	v := []byte("hello world")
	c := cid.NewCidV1(cid.Raw, v)

	t.Run("GetBlock() returns Block with duration logged", func(t *testing.T) {
		rs := new(mockBlockReader)
		blk := blocks.NewBlock(v)
		rs.On("GetBlock", mock.Anything, c).Return(blk, nil)
		b.bitswap = rs

		ctx := context.Background()

		result, err := b.GetBlock(ctx, c)
		assert.NoError(err)

		assert.Equal(blk, result)
		assert.Equal(len(b.cidDurations), 1)
	})

	t.Run("GetBlock() returns an error and records the event", func(t *testing.T) {
		rs := new(mockBlockReader)
		rs.On("GetBlock", mock.Anything, c).Return(blocks.NewBlock(v), errors.New("error"))
		b.bitswap = rs

		ctx := context.Background()

		_, err := b.GetBlock(ctx, c)
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

	bit, closer := getBitswapRetriever(t, "f01953925", false)
	defer closer()

	rs := new(mockBlockReader)
	root, nodeMap := makeDepthTestingGraph(t)

	rs.On("Close").Return(nil)
	for k, v := range nodeMap {
		fmt.Printf("mocking from nodeMap: key: %s, value: %v\n\n", k, v)
		rs.On("GetBlock", mock.Anything, k).Return(blocks.NewBlock(v.RawData()), nil)
	}

	lp2p := new(mockLibp2p)
	lp2p.On("Close").Return(nil)
	lp2p.On("Connect", mock.Anything, mock.Anything).Return(nil)

	bit.bitswap = rs
	bit.libp2p = lp2p

	t.Run("Retreive() hits deadline", func(t *testing.T) {
		ctx := context.Background()

		result, err := bit.Retrieve(ctx, root.Cid(), 8*time.Second)

		for i, s := range result.CalculatedStats.Events {
			fmt.Printf("stat-%d: %v\n", i, s)
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
	t.Skip("Only turn on for live test")
	assert := assert.New(t)

	b, closer := getBitswapRetriever(t, "f01953925", true)
	defer closer()

	c, err := cid.Decode("bafykbzaceb4gqljh5wrijjincngznnrw4f6hwjfpei4evvcwhhgh4jegjb4sy")
	assert.NoError(err)

	t.Run("GetBlock() returns Block with duration logged", func(t *testing.T) {
		result, err := b.Retrieve(context.Background(), c, 20*time.Second)
		assert.NoError(err)
		assert.Empty(result.ErrorMessage)

		fmt.Printf("result: %v\n", result)

		for i, s := range result.CalculatedStats.Events {
			fmt.Printf("stat-%d: %v\n", i+1, s)
		}

		fmt.Printf("AverageSpeedPerSec: %v\n", result.AverageSpeedPerSec)
		fmt.Printf("BytesDownloaded: %v\n", result.BytesDownloaded)
		fmt.Printf("TimeElapsed: %v\n", result.TimeElapsed)
		fmt.Printf("numberOfEvents: %v\n", len(result.Events))

		assert.NotNil(result)
		assert.Greater(len(b.cidDurations), 1)
		assert.Greater(result.BytesDownloaded, uint64(0))
	})
}
