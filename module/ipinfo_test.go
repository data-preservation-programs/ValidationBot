package module

import (
	"testing"

	"github.com/multiformats/go-multiaddr"
	"github.com/stretchr/testify/assert"
)

func TestIpInfoResolver_ResolveIP(t *testing.T) {
	assert := assert.New(t)
	resolver := IpInfoResolver{}
	country, err := resolver.ResolveIPStr("66.66.66.66")
	assert.Nil(err)
	assert.Equal("US", country)
}

func TestIpInfoResolver_ResolveIP_Local(t *testing.T) {
	assert := assert.New(t)
	resolver := IpInfoResolver{}
	country, err := resolver.ResolveIPStr("192.168.1.1")
	assert.Nil(err)
	assert.Equal("", country)
}

func TestIpInfoResolver_ResolveMultiAddr(t *testing.T) {
	assert := assert.New(t)
	resolver := IpInfoResolver{}
	addr, err := multiaddr.NewMultiaddr("/ip4/66.66.66.66/tcp/80")
	assert.Nil(err)
	country, err := resolver.ResolveMultiAddr(addr)
	assert.Nil(err)
	assert.Equal("US", country)
}

func TestIpInfoResolver_ResolveMultiAddrDNS4(t *testing.T) {
	assert := assert.New(t)
	resolver := IpInfoResolver{}
	addr, err := multiaddr.NewMultiaddr("/dns4/www.google.com/tcp/80")
	assert.Nil(err)
	country, err := resolver.ResolveMultiAddr(addr)
	assert.Nil(err)
	assert.Equal("US", country)
}
