package retrieval

import (
	"net"
	"net/url"
	"strings"

	multiaddr "github.com/multiformats/go-multiaddr"
	manet "github.com/multiformats/go-multiaddr/net"
	"github.com/pkg/errors"

	"fmt"

	"github.com/libp2p/go-libp2p/core/peer"
)

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
