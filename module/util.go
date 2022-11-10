package module

import (
	"strconv"

	"github.com/multiformats/go-multiaddr"
	"github.com/pkg/errors"
	"golang.org/x/exp/slices"
)

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
