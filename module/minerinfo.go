package module

import (
	"context"
	"fmt"
	"reflect"

	"github.com/filecoin-project/go-address"
	"github.com/filecoin-project/lotus/api"
	"github.com/filecoin-project/lotus/chain/types"
	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/multiformats/go-multiaddr"
	"github.com/pkg/errors"
	"github.com/rs/zerolog/log"
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
func GetMinerInfo(ctx context.Context, lotusAPI api.Gateway, provider string) (*MinerInfoResult, error) {
	providerAddr, err := address.NewFromString(provider)
	if err != nil {
		return &MinerInfoResult{
			ErrorCode:    InvalidProviderID,
			ErrorMessage: err.Error(),
		}, nil
	}

	log.Debug().Str("role", "lotus_api").
		Str("method", "StateMinerInfo").Str("provider", provider).Msg("calling lotus api")

	minerInfo, err := lotusAPI.StateMinerInfo(ctx, providerAddr, types.EmptyTSK)
	if err != nil {
		tp := reflect.TypeOf(err)
		if tp.String() == "*jsonrpc.respError" {
			return &MinerInfoResult{
				ErrorCode:    InvalidProviderID,
				ErrorMessage: err.Error(),
			}, nil
		}
		return nil, errors.Wrapf(err, "failed to get miner info for %s", provider)
	}

	if minerInfo.PeerId == nil {
		return &MinerInfoResult{
			ErrorCode: NoPeerID,
		}, nil
	}

	maddrs := make([]multiaddr.Multiaddr, len(minerInfo.Multiaddrs))
	maddrStrs := make([]string, len(minerInfo.Multiaddrs))

	for i, mma := range minerInfo.Multiaddrs {
		multiaddrBytes, err := multiaddr.NewMultiaddrBytes(mma)
		if err != nil {
			return &MinerInfoResult{
				ErrorCode:    InvalidMultiAddress,
				ErrorMessage: err.Error(),
			}, nil
		}

		maddrs[i] = multiaddrBytes
		maddrStrs[i] = multiaddrBytes.String()
	}

	fmt.Println(maddrStrs, minerInfo.PeerId, providerAddr, minerInfo.Multiaddrs)

	return &MinerInfoResult{
		MultiAddrs:   maddrs,
		MultiAddrStr: maddrStrs,
		PeerID:       minerInfo.PeerId,
		MinerAddress: providerAddr,
	}, nil
}
