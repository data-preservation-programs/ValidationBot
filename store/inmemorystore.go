package store

import (
	"context"

	"github.com/ipfs/go-cid"
	"github.com/libp2p/go-libp2p/core/peer"
)

type InMemoryStore struct {
	Storage [][]byte
}

//nolint:all
func (i *InMemoryStore) Head(ctx context.Context, peerID peer.ID) (*Entry, error) {
	if len(i.Storage) == 0 {
		return nil, nil
	} else {
		return &Entry{Message: i.Storage[len(i.Storage)-1]}, nil
	}
}

//nolint:all
func (i *InMemoryStore) Subscribe(ctx context.Context, peerID peer.ID, last *cid.Cid) (
	<-chan *Entry,
	error,
) {
	entryChan := make(chan *Entry)
	go func() {
		for _, entry := range i.Storage {
			entryChan <- &Entry{Message: entry}
		}
	}()
	return entryChan, nil
}

//nolint:all
func (i *InMemoryStore) Publish(ctx context.Context, input []byte) error {
	i.Storage = append(i.Storage, input)
	return nil
}
