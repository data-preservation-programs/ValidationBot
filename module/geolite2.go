package module

import (
	"net"
	"strconv"

	"validation-bot/resources"

	"github.com/multiformats/go-multiaddr"
	"github.com/oschwald/geoip2-golang"
	"github.com/pkg/errors"
	"golang.org/x/exp/slices"
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

func ResolveHostAndIP(addr multiaddr.Multiaddr) (string, bool, int, error) {
	protocols := addr.Protocols()
	isHostName := false
	const expectedProtocolCount = 2

	if len(protocols) != expectedProtocolCount {
		return "", false, 0, errors.New("multiaddr does not contain two protocols")
	}

	if !slices.Contains(
		[]int{
			multiaddr.P_IP4, multiaddr.P_IP6,
			multiaddr.P_DNS4, multiaddr.P_DNS6,
			multiaddr.P_DNS, multiaddr.P_DNSADDR,
		}, protocols[0].Code,
	) {
		return "", false, 0, errors.New("multiaddr does not contain a valid ip or dns protocol")
	}

	if slices.Contains(
		[]int{
			multiaddr.P_DNS, multiaddr.P_DNSADDR,
			multiaddr.P_DNS4, multiaddr.P_DNS6,
		}, protocols[0].Code,
	) {
		isHostName = true
	}

	if protocols[1].Code != multiaddr.P_TCP {
		return "", false, 0, errors.New("multiaddr does not contain a valid tcp protocol")
	}

	splitted := multiaddr.Split(addr)

	component0, ok := splitted[0].(*multiaddr.Component)
	if !ok {
		return "", false, 0, errors.New("failed to cast component")
	}

	host := component0.Value()

	component1, ok := splitted[1].(*multiaddr.Component)
	if !ok {
		return "", false, 0, errors.New("failed to cast component")
	}

	port, err := strconv.Atoi(component1.Value())
	if err != nil {
		return "", false, 0, errors.Wrap(err, "failed to parse port")
	}

	return host, isHostName, port, nil
}
