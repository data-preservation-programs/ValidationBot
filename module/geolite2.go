package module

import (
	"net"
	"validation-bot/resources"

	"github.com/multiformats/go-multiaddr"
	"github.com/oschwald/geoip2-golang"
	"github.com/pkg/errors"
)

type GeoLite2Resolver struct {
	db *geoip2.Reader
}

func NewGeoLite2Resolver() (*GeoLite2Resolver, error) {
	db, err := geoip2.FromBytes(resources.GeoLite2CityMmdb)
	if err != nil {
		return nil, errors.Wrap(err, "failed to open GeoLite2 database")
	}

	return &GeoLite2Resolver{
		db: db,
	}, nil
}

func (g GeoLite2Resolver) ResolveIP(ip net.IP) (*geoip2.City, error) {
	city, err := g.db.City(ip)
	if err != nil {
		return nil, errors.Wrap(err, "failed to resolve IP")
	}

	return city, nil
}

func (g GeoLite2Resolver) ResolveIPStr(ip string) (*geoip2.City, error) {
	parsed := net.ParseIP(ip)
	if parsed == nil {
		return nil, errors.Errorf("failed to parse IP address %s", ip)
	}

	return g.ResolveIP(parsed)
}

func (g GeoLite2Resolver) ResolveMultiAddr(addr multiaddr.Multiaddr) (*geoip2.City, error) {
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

	return g.ResolveIPStr(host)
}
