package module

import (
	"context"
	"reflect"

	"github.com/filecoin-project/go-address"
	"github.com/filecoin-project/lotus/api/v0api"
	"github.com/filecoin-project/lotus/chain/types"
	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/multiformats/go-multiaddr"
)

const (
	InvalidProviderID   = "invalid_provider_id"
	NoPeerID            = "no_peer_id"
	InvalidMultiAddress = "invalid_multi_address"
	NoMultiAddress      = "no_multi_address"
)

type MinerInfoResult struct {
	ErrorCode    string
	ErrorMessage string
	MultiAddrs   []multiaddr.Multiaddr
	MultiAddrStr []string
	PeerID       *peer.ID
	MinerAddress address.Address
}

//nolint:nilerr
func GetMinerInfo(ctx context.Context, lotusAPI v0api.Gateway, provider string) (*MinerInfoResult, error) {
	providerAddr, err := address.NewFromString(provider)
	if err != nil {
		return &MinerInfoResult{
			ErrorCode:    InvalidProviderID,
			ErrorMessage: err.Error(),
		}, nil
	}
	minerInfo, err := lotusAPI.StateMinerInfo(ctx, providerAddr, types.EmptyTSK)
	if err != nil {
		tp := reflect.TypeOf(err)
		if tp.String() == "*jsonrpc.respError" {
			return &MinerInfoResult{
				ErrorCode:    InvalidProviderID,
				ErrorMessage: err.Error(),
			}, nil
		}
		return nil, err
	}

	if minerInfo.PeerId == nil {
		return &MinerInfoResult{
			ErrorCode: NoPeerID,
		}, nil
	}

	maddrs := make([]multiaddr.Multiaddr, len(minerInfo.Multiaddrs))
	maddrStrs := make([]string, len(minerInfo.Multiaddrs))
	for i, mma := range minerInfo.Multiaddrs {
		ma, err := multiaddr.NewMultiaddrBytes(mma)
		if err != nil {
			return &MinerInfoResult{
				ErrorCode:    InvalidMultiAddress,
				ErrorMessage: err.Error(),
			}, nil
		}
		maddrs[i] = ma
		maddrStrs[i] = ma.String()
	}

	if len(maddrs) == 0 {
		return &MinerInfoResult{
			ErrorCode: NoMultiAddress,
		}, nil
	}

	return &MinerInfoResult{
		MultiAddrs:   maddrs,
		MultiAddrStr: maddrStrs,
		PeerID:       minerInfo.PeerId,
		MinerAddress: providerAddr,
	}, nil
}
