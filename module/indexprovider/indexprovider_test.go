package indexprovider

import (
	"context"
	"testing"

	"github.com/filecoin-project/lotus/api/client"
	"github.com/stretchr/testify/assert"
)

func TestAuditor_Enabled_Enabled(t *testing.T) {
	assert := assert.New(t)
	lotusAPI, closer, err := client.NewGatewayRPCV1(context.Background(), "https://api.node.glif.io/rpc/v0", nil)
	assert.NoError(err)
	defer closer()
	auditor := NewAuditor(lotusAPI)
	enabled, err := auditor.Enabled(context.TODO(), "f01775922")
	assert.Nil(err)
	assert.Equal(Enabled, enabled.Status)
	assert.Contains(enabled.RootCid, "ba")
}

func TestAuditor_Enabled_Disabled(t *testing.T) {
	t.Skip("skipping test for now, getting cannot_connect; cannot find another disabled provider")
	assert := assert.New(t)
	lotusAPI, closer, err := client.NewGatewayRPCV1(context.Background(), "https://api.node.glif.io/rpc/v0", nil)
	assert.NoError(err)
	defer closer()
	auditor := NewAuditor(lotusAPI)
	enabled, err := auditor.Enabled(context.TODO(), "f01761579")
	assert.Nil(err)
	assert.Equal(Disabled, enabled.Status)
}
