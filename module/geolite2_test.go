package module

import (
	"testing"

	"github.com/multiformats/go-multiaddr"
	"github.com/stretchr/testify/assert"
)

func TestGeoLite2Resolver_ResolveIP(t *testing.T) {
	assert := assert.New(t)
	resolver, err := NewGeoLite2Resolver()
	assert.Nil(err)
	city, err := resolver.ResolveIPStr("66.66.66.66")
	assert.Nil(err)
	assert.Equal("US", city.Country.IsoCode)
	assert.Equal("NA", city.Continent.Code)
	assert.Equal("NY", city.Subdivisions[0].IsoCode)
	assert.Equal("Endicott", city.City.Names["en"])
}

func TestGeoLite2Resolver_ResolveIP_Local(t *testing.T) {
	assert := assert.New(t)
	resolver, err := NewGeoLite2Resolver()
	assert.Nil(err)
	city, err := resolver.ResolveIPStr("192.168.1.1")
	assert.Nil(err)
	assert.Equal("", city.Country.IsoCode)
	assert.Equal("", city.Continent.Code)
}

func TestGeoLite2Resolver_ResolveMultiAddr(t *testing.T) {
	assert := assert.New(t)
	resolver, err := NewGeoLite2Resolver()
	assert.Nil(err)
	addr, err := multiaddr.NewMultiaddr("/ip4/66.66.66.66/tcp/80")
	assert.Nil(err)
	city, err := resolver.ResolveMultiAddr(addr)
	assert.Nil(err)
	assert.Equal("US", city.Country.IsoCode)
	assert.Equal("NA", city.Continent.Code)
	assert.Equal("NY", city.Subdivisions[0].IsoCode)
	assert.Equal("Endicott", city.City.Names["en"])
}

func TestGeoLite2Resolver_ResolveMultiAddrDNS4(t *testing.T) {
	assert := assert.New(t)
	resolver, err := NewGeoLite2Resolver()
	assert.Nil(err)
	addr, err := multiaddr.NewMultiaddr("/dns4/www.google.com/tcp/80")
	assert.Nil(err)
	city, err := resolver.ResolveMultiAddr(addr)
	assert.Nil(err)
	assert.Equal("US", city.Country.IsoCode)
	assert.Equal("NA", city.Continent.Code)
}
