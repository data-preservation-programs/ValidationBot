package retrieval

import (
	"context"
	"fmt"
	"time"

	"github.com/filecoin-project/boost/retrievalmarket/lp2pimpl"
	"github.com/filecoin-project/boost/retrievalmarket/types"
	"github.com/libp2p/go-libp2p-core/peer"
	"github.com/libp2p/go-libp2p/core/host"
	"github.com/pkg/errors"
)

func minerSupporttedProtocols(ctx context.Context, miner peer.ID, h host.Host) (*types.QueryResponse, error) {
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	client := lp2pimpl.NewTransportsClient(h)

	protocols, err := client.SendQuery(ctx, miner)
	if err != nil {
		return nil, errors.Wrap(err, fmt.Sprintf("failed to query protocols for miner %s", miner))
	}
	return protocols, nil
}
