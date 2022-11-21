package store

import (
	"context"

	"github.com/ipfs/go-cid"
	"github.com/libp2p/go-libp2p/core/peer"
)

type InMemoryStore struct {
	Storage [][]byte
}

func (i *InMemoryStore) Subscribe(ctx context.Context, peerID peer.ID, last *cid.Cid, oneOff bool) (
	<-chan Entry,
	error,
) {
	ch := make(chan Entry)
	go func() {
		for _, entry := range i.Storage {
			ch <- Entry{Message: entry}
		}
	}()
	return ch, nil
}

func (i *InMemoryStore) Publish(ctx context.Context, input []byte) error {
	i.Storage = append(i.Storage, input)
	return nil
}
