package store

import (
	"context"

	"github.com/ipfs/go-cid"
	"github.com/libp2p/go-libp2p/core/peer"
)

type ResultPublisher interface {
	Publish(ctx context.Context, input []byte) error
}

type Entry struct {
	Message []byte
	Cid     cid.Cid
}

type ResultSubscriber interface {
	Subscribe(ctx context.Context, peerID peer.ID, last *cid.Cid) (<-chan Entry, error)
}

type Store interface {
	ResultPublisher
	ResultSubscriber
}
