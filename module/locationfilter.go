package module

import (
	"github.com/oschwald/geoip2-golang"
	"golang.org/x/exp/slices"
)

type LocationFilterConfig struct {
	Continent []string
	Country   []string
}

func (l LocationFilterConfig) Match(city *geoip2.City) bool {
	if len(l.Continent) > 0 && !slices.Contains(l.Continent, city.Continent.Code) {
		return false
	}

	if len(l.Country) > 0 && !slices.Contains(l.Country, city.Country.IsoCode) {
		return false
	}

	return true
}
