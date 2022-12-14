package store

import (
	"context"

	"github.com/ipfs/go-cid"
	"github.com/libp2p/go-libp2p/core/peer"
)

type Publisher interface {
	Publish(ctx context.Context, input []byte) error
}

type Entry struct {
	Message  []byte
	Previous *cid.Cid
	CID      cid.Cid
}

type Subscriber interface {
	Head(ctx context.Context, peerID peer.ID) (*Entry, error)

	Subscribe(ctx context.Context, peerID peer.ID, last *cid.Cid) (<-chan *Entry, error)
}
