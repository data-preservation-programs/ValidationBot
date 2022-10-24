package retrieval

import (
	"context"
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/filecoin-project/lotus/api/client"
	"github.com/ipfs/go-cid"
	"github.com/rs/zerolog/log"
	"github.com/stretchr/testify/assert"
)

func TestGraphSyncRetrieverBuilderImpl_Build(t *testing.T) {
	assert := assert.New(t)
	retriever, closer := getRetriever(t)
	defer closer()
	assert.NotNil(retriever)
}

func getRetriever(t *testing.T) (GraphSyncRetriever, func()) {
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
	return retriever, func() {
		cleanup()
		closer()
	}
}

func TestGraphSync_Retrieve_NotAvailable(t *testing.T) {
	assert := assert.New(t)
	ctx := context.Background()
	retriever, cleanup := getRetriever(t)
	defer cleanup()

	c, err := cid.Decode("bafykbzacedjicdbqxgmeznb3n2uloccudvsyddlnt2w33iy4wmaafaebugrwa")
	assert.NoError(err)
	result, err := retriever.Retrieve(ctx, "f03223", c, time.Minute)
	assert.NoError(err)

	fmt.Printf("%+v\n", result)
	assert.Equal(QueryResponseError, result.Status)
	assert.Equal(GraphSync, result.Protocol)
	assert.Contains(result.ErrorMessage, "key not found")
	assert.EqualValues(0, result.BytesDownloaded)
	assert.EqualValues(0, result.TimeToFirstByte.Seconds())
	assert.EqualValues(0, result.TimeElapsed.Seconds())
}

func TestGraphSync_Retrieve_Timeout(t *testing.T) {
	if os.Getenv("SKIP_INTEGRATION") != "" {
		t.Skip("This is a real retrieval test, so it's skipped by default")
	}
	assert := assert.New(t)
	ctx := context.Background()
	retriever, cleanup := getRetriever(t)
	defer cleanup()

	c, err := cid.Decode("bafybeibxlbpejjvrfs7c3ytnpxyem5n4nkxbu7wecjcmrrawxuqojana3y")
	assert.NoError(err)
	result, err := retriever.Retrieve(ctx, "f03223", c, 10*time.Second)
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
