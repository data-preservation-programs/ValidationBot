package retrieval

import (
	"context"
	"strings"
	"validation-bot/module"

	multiaddrutil "github.com/filecoin-project/go-legs/httpsync/multiaddr"
	"github.com/libp2p/go-libp2p/core/peerstore"
	"github.com/multiformats/go-multiaddr"
	"github.com/pkg/errors"

	"fmt"

	"github.com/filecoin-project/boost/retrievalmarket/lp2pimpl"
	"github.com/filecoin-project/boost/retrievalmarket/types"
	"github.com/libp2p/go-libp2p/core/host"
	"github.com/libp2p/go-libp2p/core/peer"
)

type MinerProtocols struct {
	Protocol     Protocol
	PeerID       peer.ID
	MultiAddrs   []multiaddr.Multiaddr
	MultiAddrStr []string
}

var ErrMaxTimeReached = errors.New("dump session complete")

func GetMinerProtocols(
	ctx context.Context,
	minerInfo *module.MinerInfoResult,
	libp2p host.Host,
	proto Protocol,
) ([]MinerProtocols, error) {
	info := peer.AddrInfo{ID: *minerInfo.PeerID, Addrs: minerInfo.MultiAddrs}

	libp2p.Peerstore().AddAddrs(info.ID, info.Addrs, peerstore.PermanentAddrTTL)

	supported, err := minerSupporttedProtocols(ctx, info, libp2p)
	if err != nil && strings.Contains(err.Error(), "protocol not supported") {
		return []MinerProtocols{}, nil
	}
	if err != nil {
		return nil, errors.Wrap(err, "failed to get miner supportted protocols")
	}

	// nolint:forbidigo
	fmt.Printf("protocols: %v", supported)
	var maddrs []multiaddr.Multiaddr
	var maddrStrs []string
	var pcols []MinerProtocols

	pro := strings.ToLower(string(proto))

	for _, protocol := range supported.Protocols {
		if protocol.Name == pro {
			maddrs = make([]multiaddr.Multiaddr, len(protocol.Addresses))
			maddrStrs = make([]string, len(protocol.Addresses))
			pcols = make([]MinerProtocols, len(protocol.Addresses))

			for i, mma := range protocol.Addresses {
				multiaddrBytes, err := multiaddr.NewMultiaddrBytes(mma.Bytes())
				if err != nil {
					continue
				}

				maddrs[i] = multiaddrBytes
				if pro == "http" || pro == "https" {
					maddrStrs[i] = multiaddrToNative(protocol.Name, multiaddrBytes)
				} else {
					maddrStrs[i] = multiaddrBytes.String()
				}

				peerID, err := peerIDFromMultiAddr(mma.String())
				if err != nil {
					return nil, errors.Wrap(err, "cannot decode peer id")
				}

				libp2p.Peerstore().SetAddr(peerID, mma, peerstore.PermanentAddrTTL)

				minerp := MinerProtocols{
					Protocol:     proto,
					PeerID:       peerID,
					MultiAddrs:   maddrs,
					MultiAddrStr: maddrStrs,
				}

				// nolint:makezero
				pcols = append(pcols, minerp)
			}
		}
	}

	return pcols, nil
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
		u, err := multiaddrutil.ToURL(ma)
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
	// ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	// defer cancel()

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
