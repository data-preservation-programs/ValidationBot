package module

import (
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"strconv"
	"strings"
	"validation-bot/resources"

	"github.com/multiformats/go-multiaddr"
	"github.com/pkg/errors"
)

type GeoInfo struct {
	IP          net.IP `json:"ip"`
	City        string `json:"city"`
	Country     string `json:"country"`
	Continent   string `json:"-"`
	LocationRaw string `json:"loc"`
	Org         string `json:"org,omitempty"`

	Latitude  float64 `json:"-"`
	Longitude float64 `json:"-"`
}

type IpInfoResolver struct{}

func (i IpInfoResolver) ResolveIP(ip net.IP) (*GeoInfo, error) {
	url := fmt.Sprintf("https://ipinfo.io/%s?token=%s", ip, os.Getenv("IPINFO_TOKEN"))

	req, err := http.NewRequest("GET", url, nil)

	if err != nil {
		errors.Wrap(err, "failed to create http request")
	}

	req.Header.Set("Accept", "application/json")
	client := &http.Client{}

	resp, err := client.Do(req)

	if err != nil {
		return nil, errors.Wrap(err, "failed to resolve IP")
	}

	defer resp.Body.Close()

	geo := new(GeoInfo)

	body, err := io.ReadAll(resp.Body)

	if err != nil {
		return nil, errors.Wrap(err, "failed to read response body")
	}

	if err := json.Unmarshal(body, &geo); err != nil {
		return nil, errors.Wrap(err, "ipinfo: failed to unmarshal response")
	}

	result := strings.Split(geo.LocationRaw, ",")

	if len(result) == 2 {
		lat, long := result[0], result[1]

		geo.Latitude, err = strconv.ParseFloat(lat, 32)
		if err != nil {
			return nil, errors.Wrap(err, "failed to parse latitude")
		}

		geo.Longitude, err = strconv.ParseFloat(long, 32)
		if err != nil {
			return nil, errors.Wrap(err, "failed to parse longitude")
		}
	}

	continent := resources.CountryToContinentJson[geo.Country]

	if continent == "" {
		return nil, errors.Errorf("failed to resolve continent for country %s", geo.Country)
	}

	geo.Continent = continent

	return geo, nil
}

func (i IpInfoResolver) resolveContinent(country string) (*string, error) {
	continent := resources.CountryToContinentJson[country]

	if continent == "" {
		return nil, errors.Errorf("failed to resolve continent for country %s", country)
	}

	return &continent, nil
}

func (i IpInfoResolver) ResolveIPStr(ip string) (*GeoInfo, error) {
	parsed := net.ParseIP(ip)
	if parsed == nil {
		return nil, errors.Errorf("failed to parse IP address %s", ip)
	}

	geo, err := i.ResolveIP(parsed)

	if err != nil {
		return nil, errors.Wrap(err, "failed to resolve IP")
	}

	return geo, nil
}

func (i IpInfoResolver) ResolveMultiAddr(addr multiaddr.Multiaddr) (*GeoInfo, error) {
	host, isHostName, _, err := ResolveHostAndIP(addr)
	if err != nil {
		return nil, errors.Wrap(err, "failed to resolve host and port")
	}

	if isHostName {
		ips, err := net.LookupIP(host)
		if err != nil {
			return nil, errors.Wrapf(err, "failed to lookup host %s", host)
		}

		host = ips[0].String()
	}

	return i.ResolveIPStr(host)
}
