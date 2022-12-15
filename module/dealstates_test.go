package module

import (
	"context"
	"testing"
	"time"

	"validation-bot/helper"

	"github.com/filecoin-project/lotus/api/client"
	"github.com/stretchr/testify/assert"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

func TestDealStates(t *testing.T) {
	assert := assert.New(t)
	db, err := gorm.Open(postgres.Open(helper.PostgresConnectionString), &gorm.Config{})
	assert.Nil(err)
	assert.NotNil(db)
	lotusAPI, closer, err := client.NewGatewayRPCV1(context.Background(), "https://api.node.glif.io/rpc/v0", nil)
	assert.NoError(err)
	defer closer()
	dealStates, err := NewGlifDealStatesResolver(
		db,
		lotusAPI,
		"https://market-deal-importer.s3.us-west-2.amazonaws.com/test.json",
		time.Minute,
		2,
	)
	assert.Nil(err)
	err = dealStates.refresh(context.Background())
	assert.Nil(err)
	err = dealStates.refresh(context.Background())
	assert.Nil(err)
	deals, err := dealStates.DealsByProvider("f01895913")
	assert.Nil(err)
	assert.Equal(1, len(deals))
	assert.Equal("baga6ea4seaqauc2ydwxtamwtij6xwe7ewoxmxprdoqn4zjnuknz77v7ijewxchq", deals[0].PieceCid)
	assert.Equal("mAXCg5AIgrlxdhtWlQYd+xRf20UUMrzw+Gn9F8LogoloEJ0xWvBM", deals[0].Label)

	deals, err = dealStates.DealsByProviderClients("f01895913", []string{"f14abwn2goturifmt27s2bssoe3fup2b3npkgfzui"})
	assert.Nil(err)
	assert.Equal(1, len(deals))
	assert.Equal("baga6ea4seaqauc2ydwxtamwtij6xwe7ewoxmxprdoqn4zjnuknz77v7ijewxchq", deals[0].PieceCid)
	assert.Equal("mAXCg5AIgrlxdhtWlQYd+xRf20UUMrzw+Gn9F8LogoloEJ0xWvBM", deals[0].Label)

	deals, err = dealStates.DealsByProvider("fxxx")
	assert.Empty(deals)
	assert.Nil(err)

	deals, err = dealStates.DealsByProviderClients("fxxx", []string{"f01850099"})
	assert.Empty(deals)
	assert.Nil(err)

	deals, err = dealStates.DealsByProviderClients("f01895913", []string{"f01850090"})
	assert.Empty(deals)
	assert.Nil(err)
}
