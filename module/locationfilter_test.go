package module

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestLocationFilterConfig_Match(t *testing.T) {
	assert := assert.New(t)
	resolver, err := NewGeoLite2Resolver()
	assert.Nil(err)
	city, err := resolver.ResolveIPStr("66.66.66.66")
	assert.Nil(err)
	assert.Equal("US", city.Country.IsoCode)
	assert.Equal("NA", city.Continent.Code)

	cfg := &LocationFilterConfig{
		Continent: []string{"NA"},
	}
	cfg.Continent = []string{"NA"}
	// assert.True(cfg.Match(city.Country.IsoCode, city.Continent.Code))

	cfg = &LocationFilterConfig{}
	// assert.True(cfg.Match(city.Country.IsoCode, city.Continent.Code))

	cfg = &LocationFilterConfig{
		Country: []string{"US"},
	}
	// assert.True(cfg.Match(city.Country.IsoCode, city.Continent.Code))

	cfg = &LocationFilterConfig{
		Continent: []string{"NA"},
		Country:   []string{"CA"},
	}
	// assert.False(cfg.Match(city.Country.IsoCode, city.Continent.Code))

	cfg = &LocationFilterConfig{
		Country: []string{"CA"},
	}
	// assert.False(cfg.Match(city.Country.IsoCode, city.Continent.Code))

	cfg = &LocationFilterConfig{
		Continent: []string{"SA"},
	}
	// assert.False(cfg.Match(city.Country.IsoCode, city.Continent.Code))
}
