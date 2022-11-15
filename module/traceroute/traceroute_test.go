package traceroute

import (
	"context"
	"fmt"
	"strings"
	"testing"

	"github.com/filecoin-project/lotus/api/client"
	"github.com/stretchr/testify/assert"
)

func TestAuditor_Traceroute(t *testing.T) {
	assert := assert.New(t)
	ctx := context.TODO()
	auditor := Auditor{}
	output, err := auditor.Traceroute(ctx, "127.0.0.1", 1234, true)
	assert.Nil(err)
	fmt.Printf("%+v\n", output)
	assert.NotNil(output)
	assert.Equal(1, len(output))
	assert.Equal(1, output[0].Hop)
	assert.Equal(3, len(output[0].Probes))
	assert.Equal("127.0.0.1", output[0].Probes[0].IP)
}

func TestAuditor_ValidateProvider(t *testing.T) {
	assert := assert.New(t)
	ctx := context.TODO()
	lotusAPI, closer, err := client.NewGatewayRPCV1(context.Background(), "https://api.node.glif.io/rpc/v0", nil)
	assert.NoError(err)
	defer closer()
	auditor := NewAuditor(lotusAPI, true)
	output, err := auditor.ValidateProvider(ctx, "f03223")
	assert.Nil(err)
	fmt.Printf("%+v\n", output)
	assert.NotNil(output)
	assert.Equal(Success, output.Status)
	assert.Equal(1, len(output.Traces))
	for key, value := range output.Traces {
		assert.Equal("/ip4/69.194.1.166/tcp/12351", key)
		assert.Equal(1, value.Hops[0].Hop)
		assert.Equal(3, len(value.Hops[0].Probes))
		assert.EqualValues(0, value.LastHopOverheadMs)
	}
}

func TestAuditor_ValidateProviderWithOverhead(t *testing.T) {
	assert := assert.New(t)
	ctx := context.TODO()
	lotusAPI, closer, err := client.NewGatewayRPCV1(context.Background(), "https://api.node.glif.io/rpc/v0", nil)
	assert.NoError(err)
	defer closer()
	auditor := NewAuditor(lotusAPI, true)
	output, err := auditor.ValidateProvider(ctx, "f01732188")
	assert.Nil(err)
	fmt.Printf("%+v\n", output)
	assert.NotNil(output)
	assert.Equal(Success, output.Status)
	assert.Equal(1, len(output.Traces))
	for _, value := range output.Traces {
		assert.Equal(1, value.Hops[0].Hop)
		assert.Equal(3, len(value.Hops[0].Probes))
		assert.Greater(value.LastHopOverheadMs, float64(0))
	}
}

func TestAuditor_ValidateProvider_Evergreen(t *testing.T) {
	t.Skip("skip evergreen test")
	providers := "f01392893,f01240,f0707721,f01720359,f01786387,f01771403,f01608291,f01208862,f01199442,f01199430,f01201327,f01207045,f08403,f01611097,f022289,f039940,f03488,f024184,f030379,f033356,f0440429,f01752548,f01851231,f01775922,f09848,f023467,f01345523,f01784458,f01310564,f0773157,f01163272,f010479,f022352,f01157271,f01157288,f01208189,f01208803,f01208632,f01208154,f01157249,f01165022,f01165053,f01165159,f01165179,f01165233,f01165300,f01165428,f01165444,f01165468,f01206408,f01207874,f01207954,f01208042,f021525,f0104671,f010446,f01732189,f01852564,f0142637,f0838684,f01806491,f01224799,f01627966,f01466075,f01764587,f01790264,f01840390,f01871352,f01108096,f097777,f0461791,f01176700,f01699939,f01870135,f099608,f01127678,f01278,f01747176,f02576,f01826669,f01883179,f019551,f08399,f01801091,f014768,f0717969,f01847751,f01872811,f01882569,f01889910,f0401303,f01746964,f01864434,f01938357,f01919423,f033025,f01853438,f01423116,f01619524,f010202,f01119939,f01858429,f01861875,f01402814,f01886797,f0521569,f01367109,f01896422,f01911083,f02620,f0466405,f032824,f01910202,f0214334,f01683871,f020378,f01611281,f030384"
	assert := assert.New(t)
	ctx := context.TODO()
	lotusAPI, closer, err := client.NewGatewayRPCV1(context.Background(), "https://api.node.glif.io/rpc/v0", nil)
	assert.NoError(err)
	defer closer()
	auditor := NewAuditor(lotusAPI, true)
	for _, provider := range strings.Split(providers, ",") {
		result, err := auditor.ValidateProvider(ctx, provider)
		assert.Nil(err)
		assert.NotNil(result)
	}
}
