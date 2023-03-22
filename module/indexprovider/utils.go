package indexprovider

import (
	"context"
	"fmt"
	"io"
	"net"
	"net/http"
	"path"

	"github.com/ipfs/go-cid"
	gostream "github.com/libp2p/go-libp2p-gostream"
	"github.com/libp2p/go-libp2p/core/host"
	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/libp2p/go-libp2p/core/protocol"
	ma "github.com/multiformats/go-multiaddr"
	"github.com/multiformats/go-multistream"
	"github.com/pkg/errors"
	"github.com/rs/zerolog"
)

func QueryRootCid(ctx context.Context, host host.Host, topic string, peerID peer.ID) (cid.Cid, error) {
	log := zerolog.Ctx(ctx).With().Str("query-root-cid", topic).Logger()

	//nolint:exhaustruct
	client := http.Client{
		Transport: &http.Transport{
			DialContext: func(ctx context.Context, network, addr string) (net.Conn, error) {
				addrInfo := peer.AddrInfo{
					Addrs: []ma.Multiaddr{ma.StringCast(addr)},
					ID:    peerID,
				}
				err := host.Connect(ctx, addrInfo)
				if err != nil {
					return nil, err
				}

				derivedProtocolID := protocol.ID(path.Join("/legs/head", topic, "0.0.1"))

				conn, err := gostream.Dial(ctx, host, peerID, derivedProtocolID)
				if err != nil {
					// If protocol ID is wrong, then try the old "double-slashed" protocol ID.
					if !errors.Is(err, multistream.ErrNotSupported[protocol.ID]{Protos: []protocol.ID{derivedProtocolID}}) {
						return nil, err
					}
					oldProtoID := protocol.ID("/legs/head/" + topic + "/0.0.1")
					conn, err = gostream.Dial(ctx, host, peerID, oldProtoID)
					if err != nil {
						return nil, err
					}
					log.Warn().Str("protocol", string(oldProtoID)).Msg("using old protocol ID")
				}
				return conn, err
			},
		},
	}

	// The httpclient expects there to be a host here. `.invalid` is a reserved
	// TLD for this purpose. See
	// https://datatracker.ietf.org/doc/html/rfc2606#section-2
	// nolint:noctx
	resp, err := client.Get("http://unused.invalid/head")
	if err != nil {
		return cid.Undef, err
	}
	defer resp.Body.Close()

	cidStr, err := io.ReadAll(resp.Body)
	if err != nil {
		return cid.Undef, fmt.Errorf("cannot fully read response body: %w", err)
	}
	if len(cidStr) == 0 {
		log.Debug().Msg("No head is set; returning cid.Undef")
		return cid.Undef, nil
	}

	cs := string(cidStr)
	decode, err := cid.Decode(cs)
	if err != nil {
		return cid.Undef, fmt.Errorf("failed to decode CID %s: %w", cs, err)
	}

	log.Debug().Str("cid", decode.String()).Msg("got root CID for latest head")
	return decode, nil
}
