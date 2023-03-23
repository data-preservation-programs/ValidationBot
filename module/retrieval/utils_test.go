package retrieval

import (
	"context"
	"reflect"
	"testing"

	multiaddr "github.com/multiformats/go-multiaddr"
	"github.com/pkg/errors"

	"github.com/filecoin-project/boost/retrievalmarket/lp2pimpl"
	"github.com/filecoin-project/boost/retrievalmarket/types"
	"github.com/libp2p/go-libp2p/core/peer"
)

type mockHost struct{}

func (h *mockHost) Connect(ctx context.Context, p peer.AddrInfo) error {
	return nil
}

type mockTransportClient struct {
	protocols *types.QueryResponse
	err       error
}

func (t *mockTransportClient) SendQuery(ctx context.Context, p peer.ID) (*types.QueryResponse, error) {
	return t.protocols, t.err
}

func TestGetMinerProtocols(t *testing.T) {
	t.Parallel()

	ma1, err := multiaddr.NewMultiaddr("/ip4/1.2.3.4/tcp/80")
	if err != nil {
		t.Fatalf("failed to create multiaddr: %v", err)
	}

	ma2, err := multiaddr.NewMultiaddr("/ip4/1.2.3.4/tcp/81")
	if err != nil {
		t.Fatalf("failed to create multiaddr: %v", err)
	}

	info := peer.AddrInfo{
		ID:    peer.ID("abc"),
		Addrs: []multiaddr.Multiaddr{ma1, ma2},
	}

	libp2p := &mockHost{}

	response := &types.QueryResponse{
		Protocols: []types.Protocol{
			{Name: HTTP, Addresses: []multiaddr.Multiaddr{ma1}},
			{Name: HTTPS, Addresses: []multiaddr.Multiaddr{ma2}},
			{Name: Libp2p, Addresses: []multiaddr.Multiaddr{ma1, ma2}},
			{Name: WS, Addresses: []multiaddr.Multiaddr{ma2}},
			{Name: WSS, Addresses: []multiaddr.Multiaddr{ma1}},
			{Name: BitswapProto, Addresses: []multiaddr.Multiaddr{ma1, ma2}},
			{Name: "unsupported", Addresses: []multiaddr.Multiaddr{ma1, ma2}},
		},
	}

	cases := []struct {
		name           string
		client         lp2pimpl.TransportsClient
		expectedResult []MinerProtocols
		expectedError  error
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
					MultiAddrStr: []string{ma1.String()},
				},
				{
					Protocol:     response.Protocols[1],
					PeerID:       "",
					MultiAddrs:   []multiaddr.Multiaddr{ma2},
					MultiAddrStr: []string{ma2.String()},
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
					PeerID:       peer.ID(""),
					MultiAddrs:   []multiaddr.Multiaddr{ma1, ma2},
					MultiAddrStr: []string{ma1.String(), ma2.String()},
				},
				{
					Protocol:     response.Protocols[6],
					PeerID:       peer.ID(""),
					MultiAddrs:   []multiaddr.Multiaddr{ma1, ma2},
					MultiAddrStr: []string{ma1.String(), ma2.String()},
				},
			},
			expectedError: nil,
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
			expectedError:  nil,
		},
		{
			name: "failed query with error containing 'protocol not supported'",
			client: &mockTransportClient{
				protocols: nil,
				err:       errors.New("protocol not supported"),
			},
			expectedResult: []MinerProtocols{},
			expectedError:  nil,
		},
		{
			name: "failed query with generic error",
			client: &mockTransportClient{
				protocols: nil,
				err:       errors.New("failed to query protocols"),
			},
			expectedResult: nil,
			expectedError:  errors.Wrap(errors.New("failed to query protocols"), "failed to get miner supportted protocols"),
		},
	}

	for _, c := range cases {
		c := c

		t.Run(c.name, func(t *testing.T) {
			result, err := GetMinerProtocols(context.Background(), info, libp2p, c.client)

			if !reflect.DeepEqual(result, c.expectedResult) {
				t.Errorf("expected result %v, but got %v", c.expectedResult, result)
			}

			if !errors.Is(err, c.expectedError) {
				t.Errorf("expected error %v, but got %v", c.expectedError, err)
			}
		})
	}

}
