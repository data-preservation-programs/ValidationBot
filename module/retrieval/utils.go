package retrieval

import (
	"context"
	"net"
	"net/url"
	"strings"
	"time"

	multiaddr "github.com/multiformats/go-multiaddr"
	manet "github.com/multiformats/go-multiaddr/net"
	"github.com/pkg/errors"

	"fmt"

	"github.com/filecoin-project/boost/retrievalmarket/lp2pimpl"
	"github.com/filecoin-project/boost/retrievalmarket/types"
	"github.com/libp2p/go-libp2p/core/host"
	"github.com/libp2p/go-libp2p/core/peer"
)

const (
	HTTP         = "http"
	HTTPS        = "https"
	Libp2p       = "libp2p"
	WS           = "ws"
	WSS          = "wss"
	BitswapProto = "bitswap"
)

type MinerProtocols struct {
	Protocol     types.Protocol
	PeerID       peer.ID
	MultiAddrs   []multiaddr.Multiaddr
	MultiAddrStr []string
}

var ErrMaxTimeReached = errors.New("dump session complete")

func GetMinerProtocols(
	ctx context.Context,
	info peer.AddrInfo,
	libp2p host.Host,
) ([]MinerProtocols, error) {
	supported, err := minerSupporttedProtocols(ctx, info, libp2p)
	if err != nil && strings.Contains(err.Error(), "protocol not supported") {
		return []MinerProtocols{}, nil
	}
	if err != nil {
		return nil, errors.Wrap(err, "failed to get miner supportted protocols")
	}

	var protocols []MinerProtocols

	for _, protocol := range supported.Protocols {
		maddrs := make([]multiaddr.Multiaddr, len(supported.Protocols))
		maddrStrs := make([]string, len(supported.Protocols))
		protocols = make([]MinerProtocols, len(protocol.Addresses))
		var peerID peer.ID

		for i, mma := range protocol.Addresses {
			multiaddrBytes, err := multiaddr.NewMultiaddrBytes(mma.Bytes())
			if err != nil {
				continue
			}

			switch protocol.Name {
			case HTTP, HTTPS, Libp2p, WS, WSS:
				maddrs[i] = multiaddrBytes
				maddrStrs[i] = multiaddrToNative(protocol.Name, multiaddrBytes)
			case BitswapProto:
				maddrs[i] = mma
				maddrStrs[i] = mma.String()

				peerID, err = peerIDFromMultiAddr(mma.String())
				if err != nil {
					return nil, errors.Wrap(err, "cannot decode peer id")
				}
			default:
				// do nothing right now
			}

			minerp := MinerProtocols{
				Protocol:     protocol,
				PeerID:       peerID,
				MultiAddrs:   maddrs,
				MultiAddrStr: maddrStrs,
			}

			// nolint:makezero
			protocols = append(protocols, minerp)
		}
	}

	return protocols, nil
}

func peerIDFromMultiAddr(ma string) (peer.ID, error) {
	split := strings.Split(ma, "/")
	id := split[len(split)-1]

	peerID, err := peer.Decode(id)
	if err != nil {
		return "", errors.Wrap(err, "failed to decode peer id")
	}

	return peerID, nil
}

// ToURL takes a multiaddr of the form:
// taken from https://github.com/filecoin-project/go-legs/blame/main/httpsync/multiaddr/convert.go#L43-L84.
// /dns/thing.com/http/urlescape<path/to/root>.
// /ip/192.168.0.1/tcp/80/http.
func ToURL(ma multiaddr.Multiaddr) (*url.URL, error) {
	// host should be either the dns name or the IP
	_, host, err := manet.DialArgs(ma)
	if err != nil {
		return nil, errors.Wrap(err, "failed to get dial args")
	}
	if ip := net.ParseIP(host); ip != nil {
		if !ip.To4().Equal(ip) {
			// raw v6 IPs need `[ip]` encapsulation.
			host = fmt.Sprintf("[%s]", host)
		}
	}

	protos := ma.Protocols()
	pm := make(map[int]string, len(protos))
	for _, p := range protos {
		v, err := ma.ValueForProtocol(p.Code)
		if err == nil {
			pm[p.Code] = v
		}
	}

	scheme := HTTP
	//nolint:nestif
	if _, ok := pm[multiaddr.P_HTTPS]; ok {
		scheme = HTTPS
	} else if _, ok = pm[multiaddr.P_HTTP]; ok {
		// /tls/http == /https
		if _, ok = pm[multiaddr.P_TLS]; ok {
			scheme = HTTPS
		}
	} else if _, ok = pm[multiaddr.P_WSS]; ok {
		scheme = WSS
	} else if _, ok = pm[multiaddr.P_WS]; ok {
		scheme = WS
		// /tls/ws == /wss
		if _, ok = pm[multiaddr.P_TLS]; ok {
			scheme = WSS
		}
	}

	path := ""
	if pb, ok := pm[0x300200]; ok {
		path, err = url.PathUnescape(pb)
		if err != nil {
			path = ""
		}
	}

	//nolint:exhaustruct
	out := url.URL{
		Scheme: scheme,
		Host:   host,
		Path:   path,
	}
	return &out, nil
}

func multiaddrToNative(proto string, ma multiaddr.Multiaddr) string {
	switch proto {
	case HTTP, HTTPS:
		u, err := ToURL(ma)
		if err != nil {
			return ""
		}
		return u.String()
	}

	return ""
}

func minerSupporttedProtocols(
	ctx context.Context,
	minerInfo peer.AddrInfo,
	host host.Host,
) (*types.QueryResponse, error) {
	// nolint:gomnd
	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	id, err := peer.Decode(minerInfo.ID.String())
	if err != nil {
		return nil, fmt.Errorf("failed to decode peer id %s: %w", minerInfo.ID, err)
	}

	addrInfo := peer.AddrInfo{ID: id, Addrs: minerInfo.Addrs}
	if err := host.Connect(ctx, addrInfo); err != nil {
		return nil, fmt.Errorf("failed to connect to peer %s: %w", addrInfo.ID, err)
	}

	client := lp2pimpl.NewTransportsClient(host)
	protocols, err := client.SendQuery(ctx, addrInfo.ID)
	if err != nil {
		return nil, errors.Wrap(err, fmt.Sprintf("failed to query protocols for miner %s", minerInfo))
	}
	return protocols, nil
}
