package retrieval

import (
	"context"
	"strings"
	"time"
	"validation-bot/module"

	multiaddrutil "github.com/filecoin-project/go-legs/httpsync/multiaddr"
	multiaddr "github.com/multiformats/go-multiaddr"
	"github.com/pkg/errors"

	"fmt"

	"github.com/filecoin-project/boost/retrievalmarket/lp2pimpl"
	"github.com/filecoin-project/boost/retrievalmarket/types"
	"github.com/libp2p/go-libp2p/core/host"
	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/libp2p/go-libp2p/core/peerstore"
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
	minerInfo *module.MinerInfoResult,
	libp2p host.Host,
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

				libp2p.Peerstore().AddAddr(info.ID, mma, peerstore.PermanentAddrTTL)
			default:
				// do nothing right now
			}

			// fmt.Println(libp2p.Peerstore().Addrs(info.ID))

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
