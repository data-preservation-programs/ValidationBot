package module

import (
	"golang.org/x/exp/slices"
)

type LocationFilterConfig struct {
	Continent []string
	Country   []string
}

func (l LocationFilterConfig) Match(continent string, country string) bool {
	if len(l.Continent) > 0 && !slices.Contains(l.Continent, continent) {
		return false
	}

	if len(l.Country) > 0 && !slices.Contains(l.Country, country) {
		return false
	}

	return true
}
