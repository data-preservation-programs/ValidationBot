package store

import (
	"context"

	"github.com/ipfs/go-cid"
)

type ResultPublisher interface {
	Publish(ctx context.Context, input []byte) error
}

type Entry struct {
	Message []byte
	Cid     cid.Cid
}

type ResultSubscriber interface {
	Subscribe(ctx context.Context, last cid.Cid) error
	Next(ctx context.Context) (Entry, error)
}

type Store interface {
	ResultPublisher
	ResultSubscriber
}
