package module

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestLocationFilterConfig_Match(t *testing.T) {
	assert := assert.New(t)
	resolver, err := NewIPInfoResolver()
	assert.Nil(err)
	countryCode, err := resolver.ResolveIPStr(context.TODO(), "66.66.66.66")
	assert.Nil(err)
	continentCode := resolver.Continents[countryCode]

	assert.Equal("US", countryCode)
	assert.Equal("NA", continentCode)

	cfg := &LocationFilterConfig{
		Continent: []string{"NA"},
	}
	cfg.Continent = []string{"NA"}
	assert.True(cfg.Match(countryCode, continentCode))

	cfg = &LocationFilterConfig{}
	assert.True(cfg.Match(countryCode, continentCode))

	cfg = &LocationFilterConfig{
		Country: []string{"US"},
	}
	assert.True(cfg.Match(countryCode, continentCode))

	cfg = &LocationFilterConfig{
		Continent: []string{"NA"},
		Country:   []string{"CA"},
	}
	assert.False(cfg.Match(countryCode, continentCode))

	cfg = &LocationFilterConfig{
		Country: []string{"CA"},
	}
	assert.False(cfg.Match(countryCode, continentCode))

	cfg = &LocationFilterConfig{
		Continent: []string{"SA"},
	}
	assert.False(cfg.Match(countryCode, continentCode))
}
