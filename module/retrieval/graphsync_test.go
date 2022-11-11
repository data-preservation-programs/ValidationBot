package retrieval

import (
	"context"
	"fmt"
	"os"
	"testing"
	"time"

	"validation-bot/module"

	"github.com/filecoin-project/lotus/api"
	"github.com/filecoin-project/lotus/api/client"
	"github.com/ipfs/go-cid"
	"github.com/rs/zerolog/log"
	"github.com/stretchr/testify/assert"
)

func TestGraphSyncRetrieverBuilderImpl_Build(t *testing.T) {
	assert := assert.New(t)
	retriever, _, closer := getRetriever(t)
	defer closer()
	assert.NotNil(retriever)
}

func getRetriever(t *testing.T) (GraphSyncRetriever, api.Gateway, func()) {
	assert := assert.New(t)
	ctx := context.Background()
	lotusAPI, closer, err := client.NewGatewayRPCV1(ctx, "https://api.node.glif.io/rpc/v0", nil)
	assert.NoError(err)
	builder := GraphSyncRetrieverBuilderImpl{
		LotusAPI: lotusAPI,
		BaseDir:  os.TempDir(),
	}
	retriever, cleanup, err := builder.Build()
	assert.NoError(err)
	assert.NotNil(retriever)
	return retriever, lotusAPI, func() {
		cleanup()
		closer()
	}
}

func TestGraphSync_Retrieve_NotAvailable(t *testing.T) {
	assert := assert.New(t)
	ctx := context.Background()
	retriever, lotusAPI, cleanup := getRetriever(t)
	defer cleanup()

	c, err := cid.Decode("bafykbzacedjicdbqxgmeznb3n2uloccudvsyddlnt2w33iy4wmaafaebugrwa")
	assert.NoError(err)
	minerInfo, err := module.GetMinerInfo(ctx, lotusAPI, "f03223")
	assert.NoError(err)

	result, err := retriever.Retrieve(ctx, minerInfo.MinerAddress, c, time.Minute)
	assert.NoError(err)

	fmt.Printf("%+v\n", result)
	assert.Equal(QueryResponseError, result.Status)
	assert.Equal(GraphSync, result.Protocol)
	assert.Contains(result.ErrorMessage, "key not found")
	assert.EqualValues(0, result.BytesDownloaded)
	assert.EqualValues(0, result.TimeToFirstByte.Seconds())
	assert.Less(result.TimeElapsed.Seconds(), float64(1))
}

func TestGraphSync_Retrieve_Timeout(t *testing.T) {
	t.Skip("This is a real retrieval test, so it's skipped by default")
	assert := assert.New(t)
	ctx := context.Background()
	retriever, lotusAPI, cleanup := getRetriever(t)
	defer cleanup()

	c, err := cid.Decode("bafybeibxlbpejjvrfs7c3ytnpxyem5n4nkxbu7wecjcmrrawxuqojana3y")
	assert.NoError(err)
	minerInfo, err := module.GetMinerInfo(ctx, lotusAPI, "f03223")
	assert.NoError(err)
	result, err := retriever.Retrieve(ctx, minerInfo.MinerAddress, c, 10*time.Second)
	assert.NoError(err)

	fmt.Printf("%+v\n", result)
	assert.Equal(RetrieveTimeout, result.Status)
	assert.Greater(result.BytesDownloaded, uint64(0))
	assert.Greater(result.AverageSpeedPerSec, float64(0))
	assert.Greater(result.TimeElapsed.Seconds(), float64(0))
	assert.Greater(result.TimeToFirstByte.Seconds(), float64(0))
}

func TestRetrievalStats_NewResultContent(t *testing.T) {
	assert := assert.New(t)
	stats := retrievalStats{
		log:    log.With().Logger(),
		done:   make(chan interface{}),
		events: []TimeEventPair{},
	}
	stats.events = append(stats.events, TimeEventPair{})
	result := stats.NewResultContent(QueryFailure, "query failed")
	assert.Equal(QueryFailure, result.Status)
	assert.Equal("query failed", result.ErrorMessage)
}
