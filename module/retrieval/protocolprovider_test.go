package retrieval

import (
	"context"
	"fmt"
	"reflect"
	"testing"
	"validation-bot/role"

	multiaddr "github.com/multiformats/go-multiaddr"

	"github.com/filecoin-project/boost/retrievalmarket/types"
	"github.com/libp2p/go-libp2p/core/peer"
)

type mockTransportClient struct {
	protocols *types.QueryResponse
	err       error
}

func (t *mockTransportClient) SendQuery(ctx context.Context, p peer.ID) (*types.QueryResponse, error) {
	return t.protocols, t.err
}

func TestGetMinerProtocols(t *testing.T) {
	t.Parallel()

	ma1, err := multiaddr.NewMultiaddr("/ip4/127.0.0.1/tcp/1234/")
	if err != nil {
		t.Fatalf("failed to create multiaddr: %v", err)
	}

	ma2, err := multiaddr.NewMultiaddr("/ip4/127.0.0.1/udp/1234/quic")
	if err != nil {
		t.Fatalf("failed to create multiaddr: %v", err)
	}

	_, _, pid, err := role.GenerateNewPeer()
	if err != nil {
		t.Fatalf("failed to generate peer: %v", err)
	}

	bma, err := multiaddr.NewMultiaddr(fmt.Sprintf("/ip4/209.94.92.6/udp/24123/quic/p2p/%s", pid.String()))
	if err != nil {
		t.Fatalf("failed to create multiaddr: %v", err)
	}

	info := peer.AddrInfo{
		ID:    pid,
		Addrs: []multiaddr.Multiaddr{ma1, ma2},
	}

	libp2p := &mockHost{}

	response := &types.QueryResponse{
		Protocols: []types.Protocol{
			{Name: string(HTTP), Addresses: []multiaddr.Multiaddr{ma1}},
			{Name: string(HTTPS), Addresses: []multiaddr.Multiaddr{ma2}},
			{Name: string(Libp2p), Addresses: []multiaddr.Multiaddr{ma1, ma2}},
			{Name: string(WS), Addresses: []multiaddr.Multiaddr{ma2}},
			{Name: string(WSS), Addresses: []multiaddr.Multiaddr{ma1}},
			{Name: string(Bitswap), Addresses: []multiaddr.Multiaddr{bma}},
		},
	}

	cases := []struct {
		name           string
		client         TransportClient
		expectedResult []MinerProtocols
	}{
		{
			name: "successful query with all protocols",
			client: &mockTransportClient{
				protocols: response,
				err:       nil,
			},
			expectedResult: []MinerProtocols{
				{
					Protocol:     response.Protocols[0],
					PeerID:       "",
					MultiAddrs:   []multiaddr.Multiaddr{ma1},
					MultiAddrStr: []string{multiaddrToNative(string(HTTP), ma1)},
				},
				{
					Protocol:     response.Protocols[1],
					PeerID:       "",
					MultiAddrs:   []multiaddr.Multiaddr{ma2},
					MultiAddrStr: []string{multiaddrToNative(string(HTTPS), ma2)},
				},
				{
					Protocol:     response.Protocols[2],
					PeerID:       "",
					MultiAddrs:   []multiaddr.Multiaddr{ma1, ma2},
					MultiAddrStr: []string{ma1.String(), ma2.String()},
				},
				{
					Protocol:     response.Protocols[3],
					PeerID:       "",
					MultiAddrs:   []multiaddr.Multiaddr{ma2},
					MultiAddrStr: []string{ma2.String()},
				},
				{
					Protocol:     response.Protocols[4],
					PeerID:       "",
					MultiAddrs:   []multiaddr.Multiaddr{ma1},
					MultiAddrStr: []string{ma1.String()},
				},
				{
					Protocol:     response.Protocols[5],
					PeerID:       pid,
					MultiAddrs:   []multiaddr.Multiaddr{bma},
					MultiAddrStr: []string{bma.String()},
				},
			},
		},
		{
			name: "successful query with no supported protocols",
			client: &mockTransportClient{
				protocols: &types.QueryResponse{
					Protocols: []types.Protocol{},
				},
				err: nil,
			},
			expectedResult: []MinerProtocols{},
		},
	}

	for _, c := range cases {
		c := c

		t.Run(c.name, func(t *testing.T) {
			libp2p.On("Connect", context.Background(), info).Return(nil)
			provider := NewProtocolProvider(libp2p)

			provider.client = c.client

			result, err := provider.GetMinerProtocols(context.Background(), info)
			if err != nil {
				t.Errorf("failed to get miner protocols: %v", err)
			}

			if len(result) != len(c.expectedResult) {
				t.Errorf("length mismatch expected result length: %v\n, result length: %v\n", len(c.expectedResult), len(result))
			}

			for i, r := range result {
				if !reflect.DeepEqual(r, c.expectedResult[i]) {
					t.Errorf("expected result %v\n, but got %v\n", c.expectedResult[i], r)
				}
			}
		})
	}
}
