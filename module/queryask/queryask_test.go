package queryask

import (
	"context"
	"fmt"
	"testing"
	"validation-bot/role"

	"github.com/filecoin-project/lotus/api/client"
	"github.com/pkg/errors"
	"github.com/stretchr/testify/assert"
)

func getModule(t *testing.T) QueryAsk {
	assert := assert.New(t)
	priv, _, _, err := role.GenerateNewPeer()
	assert.Nil(err)
	libp2p, err := role.NewLibp2pHost(priv, "/ip4/0.0.0.0/tcp/0")
	assert.Nil(err)
	assert.NotNil(libp2p)
	ctx := context.Background()
	lotusApi, closer, err := client.NewGatewayRPCV0(ctx, "https://api.node.gif.io/", nil)
	defer closer()
	assert.Nil(err)
	queryAsk := NewQueryAskModule(*libp2p, lotusApi)
	return queryAsk
}

func TestQueryAsk_QueryMiner_InvalidProviderId(t *testing.T) {
	assert := assert.New(t)
	queryAsk := getModule(t)
	assert.NotNil(queryAsk)
	ctx := context.Background()
	result := queryAsk.QueryMiner(ctx, "xxxxxx")
	assert.Equal(result.status, InvalidProviderId)
}

func TestQueryAsk_QueryMiner_InvalidProviderId2(t *testing.T) {
	assert := assert.New(t)
	queryAsk := getModule(t)
	assert.NotNil(queryAsk)
	ctx := context.Background()
	result := queryAsk.QueryMiner(ctx, "f010000")
	fmt.Printf("result: %+v\n", result)
	assert.Equal(result.status, InvalidProviderId)
	errors.Is()
}
