package module

import (
	"context"
	"testing"
	"time"

	"github.com/filecoin-project/lotus/api/client"
	"github.com/stretchr/testify/assert"
)

func TestDealStates(t *testing.T) {
	assert := assert.New(t)
	lotusAPI, closer, err := client.NewGatewayRPCV1(context.Background(), "https://api.node.glif.io/rpc/v0", nil)
	assert.NoError(err)
	defer closer()
	dealStates, err := NewDealStatesResolver(
		context.Background(),
		lotusAPI,
		"https://market-deal-importer.s3.us-west-2.amazonaws.com/test.json",
		time.Minute,
	)
	assert.Nil(err)
	err = dealStates.refresh(context.Background())
	assert.Nil(err)
	deals := dealStates.DealsByProvider("f01895913")
	assert.Equal(6, len(deals))
	assert.Contains(deals[0].PieceCID, "baga")
	assert.Contains(deals[0].Label, "mAX")

	deals, err = dealStates.DealsByProviderClient("f01895913", "f01850099")
	assert.Nil(err)
	assert.Equal(6, len(deals))
	assert.Contains(deals[0].PieceCID, "baga")
	assert.Contains(deals[0].Label, "mAX")

	deals, err = dealStates.DealsByProviderClients("f01895913", []string{"f14abwn2goturifmt27s2bssoe3fup2b3npkgfzui"})
	assert.Nil(err)
	assert.Equal(6, len(deals))
	assert.Contains(deals[0].PieceCID, "baga")
	assert.Contains(deals[0].Label, "mAX")

	assert.Empty(dealStates.DealsByProvider("fxxx"))
	assert.Empty(dealStates.DealsByProviderClient("fxxx", "f01850099"))
	assert.Empty(dealStates.DealsByProviderClient("f01895913", "fxxx"))
}
