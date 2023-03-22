package retrieval

import (
	"bytes"
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

func pathVal(b []byte) error {
	if bytes.IndexByte(b, '/') >= 0 {
		return fmt.Errorf("encoded path '%s' contains a slash", string(b))
	}
	return nil
}

func pathStB(s string) ([]byte, error) {
	return []byte(s), nil
}

func pathBtS(b []byte) (string, error) {
	return string(b), nil
}

// register an 'httpath' component:
var transcodePath = multiaddr.NewTranscoderFromFunctions(pathStB, pathBtS, pathVal)

func init() {
	_ = multiaddr.AddProtocol(protoHTTPath)
}

var protoHTTPath = multiaddr.Protocol{
	Name:       "httpath",
	Code:       0x300200,
	VCode:      multiaddr.CodeToVarint(0x300200),
	Size:       multiaddr.LengthPrefixedVarSize,
	Transcoder: transcodePath,
}

// ToURL takes a multiaddr of the form:
// /dns/thing.com/http/urlescape<path/to/root>
// /ip/192.168.0.1/tcp/80/http
func ToURL(ma multiaddr.Multiaddr) (*url.URL, error) {
	// host should be either the dns name or the IP
	_, host, err := manet.DialArgs(ma)
	if err != nil {
		return nil, err
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

	scheme := "http"
	if _, ok := pm[multiaddr.P_HTTPS]; ok {
		scheme = "https"
	} else if _, ok = pm[multiaddr.P_HTTP]; ok {
		// /tls/http == /https
		if _, ok = pm[multiaddr.P_TLS]; ok {
			scheme = "https"
		}
	} else if _, ok = pm[multiaddr.P_WSS]; ok {
		scheme = "wss"
	} else if _, ok = pm[multiaddr.P_WS]; ok {
		scheme = "ws"
		// /tls/ws == /wss
		if _, ok = pm[multiaddr.P_TLS]; ok {
			scheme = "wss"
		}
	}

	path := ""
	if pb, ok := pm[protoHTTPath.Code]; ok {
		path, err = url.PathUnescape(pb)
		if err != nil {
			path = ""
		}
	}

	out := url.URL{
		Scheme: scheme,
		Host:   host,
		Path:   path,
	}
	return &out, nil
}

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
			case "http", "https", "libp2p", "ws", "wss":
				maddrs[i] = multiaddrBytes
				maddrStrs[i] = multiaddrToNative(protocol.Name, multiaddrBytes)
			case "bitswap":
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

func multiaddrToNative(proto string, ma multiaddr.Multiaddr) string {
	switch proto {
	case "http", "https":
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
