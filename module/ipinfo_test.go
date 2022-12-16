package module

import (
	"context"
	"testing"

	"github.com/multiformats/go-multiaddr"
	"github.com/stretchr/testify/assert"
)

func TestIPInfoResolver_ResolveIP(t *testing.T) {
	assert := assert.New(t)
	resolver := IPInfoResolver{}
	country, err := resolver.ResolveIPStr(context.TODO(), "66.66.66.66")
	assert.Nil(err)
	assert.Equal("US", country)
}

func TestIPInfoResolver_ResolveIP_Local(t *testing.T) {
	assert := assert.New(t)
	resolver := IPInfoResolver{}
	country, err := resolver.ResolveIPStr(context.TODO(), "192.168.1.1")
	assert.Nil(err)
	assert.Equal("", country)
}

func TestIPInfoResolver_ResolveMultiAddr(t *testing.T) {
	assert := assert.New(t)
	resolver := IPInfoResolver{}
	addr, err := multiaddr.NewMultiaddr("/ip4/66.66.66.66/tcp/80")
	assert.Nil(err)
	country, err := resolver.ResolveMultiAddr(context.TODO(), addr)
	assert.Nil(err)
	assert.Equal("US", country)
}

func TestIPInfoResolver_ResolveMultiAddrDNS4(t *testing.T) {
	assert := assert.New(t)
	resolver := IPInfoResolver{}
	addr, err := multiaddr.NewMultiaddr("/dns4/www.google.com/tcp/80")
	assert.Nil(err)
	country, err := resolver.ResolveMultiAddr(context.TODO(), addr)
	assert.Nil(err)
	assert.Equal("US", country)
}
