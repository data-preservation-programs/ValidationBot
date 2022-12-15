package module

import (
	"golang.org/x/exp/slices"
)

type LocationFilterConfig struct {
	Continent []string
	Country   []string
}

func (l LocationFilterConfig) Match(countryCode string, continentCode string) bool {
	if len(l.Continent) > 0 && !slices.Contains(l.Continent, continentCode) {
		return false
	}

	if len(l.Country) > 0 && !slices.Contains(l.Country, countryCode) {
		return false
	}

	return true
}
