package resources

import _ "embed"

//go:embed GeoLite2-City.mmdb
var GeoLite2CityMmdb []byte

//go:embed country-to-continent.json
var CountryToContinentJson []byte
