package retrieval

import (
	"context"
	"fmt"
	"time"

	"github.com/filecoin-project/boost/retrievalmarket/lp2pimpl"
	"github.com/filecoin-project/boost/retrievalmarket/types"
	"github.com/libp2p/go-libp2p/core/host"
	"github.com/libp2p/go-libp2p/core/peer"
	multiaddr "github.com/multiformats/go-multiaddr"
	"github.com/pkg/errors"
)

// simple Libp2p interface to allow for testing (bypassing Connect on mocks)
type Libp2pish interface {
	host.Host
	Connect(ctx context.Context, addrInfo peer.AddrInfo) error
	Close() error
}

// interface for mocking lp2pimpl.TransportClient
type TransportClient interface {
	SendQuery(ctx context.Context, p peer.ID) (*types.QueryResponse, error)
}

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

type ProtocolProvider struct {
	host   Libp2pish
	client TransportClient
}

func NewProtocolProvider(host Libp2pish) *ProtocolProvider {
	client := lp2pimpl.NewTransportsClient(host)

	return &ProtocolProvider{
		host:   host,
		client: client,
	}
}

func (p *ProtocolProvider) GetMinerProtocols(ctx context.Context, minerInfo peer.AddrInfo) ([]MinerProtocols, error) {
	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	protocols, err := p.GetRawProtocols(ctx, minerInfo)
	if err != nil {
		return nil, errors.Wrap(err, "failed to get protocols")
	}

	minerprotos, err := p.formatMinerProtocols(ctx, protocols)
	if err != nil {
		return nil, errors.Wrap(err, "failed to format protocols")
	}

	return minerprotos, nil
}

func (p *ProtocolProvider) GetRawProtocols(
	ctx context.Context,
	minerInfo peer.AddrInfo,
) (*types.QueryResponse, error) {
	// nolint:gomnd
	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	id, err := peer.Decode(minerInfo.ID.String())
	if err != nil {
		return nil, fmt.Errorf("failed to decode peer id %s: %w", minerInfo.ID, err)
	}

	addrInfo := peer.AddrInfo{ID: id, Addrs: minerInfo.Addrs}
	// TODO do we want to defer host.Close()?
	if err := p.host.Connect(ctx, addrInfo); err != nil {
		return nil, fmt.Errorf("failed to connect to peer %s: %w", addrInfo.ID, err)
	}

	protocols, err := p.client.SendQuery(ctx, addrInfo.ID)
	if err != nil {
		return nil, errors.Wrap(err, fmt.Sprintf("failed to query protocols for miner %s", minerInfo))
	}

	return protocols, nil
}

func (p *ProtocolProvider) formatMinerProtocols(
	ctx context.Context,
	protos *types.QueryResponse,
) ([]MinerProtocols, error) {
	var protocols []MinerProtocols

	for _, protocol := range protos.Protocols {
		maddrs := make([]multiaddr.Multiaddr, len(protos.Protocols))
		maddrStrs := make([]string, len(protos.Protocols))
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

			minerprotos := MinerProtocols{
				Protocol:     protocol,
				PeerID:       peerID,
				MultiAddrs:   maddrs,
				MultiAddrStr: maddrStrs,
			}

			// nolint:makezero
			protocols = append(protocols, minerprotos)
		}
	}

	return protocols, nil
}
