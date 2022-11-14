package trust

import (
	"context"
	"testing"
	"validation-bot/helper"
	"validation-bot/store"

	"github.com/ipfs/go-cid"
	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/stretchr/testify/assert"
)

type InMemoryStore struct {
	Storage [][]byte
}

func (i *InMemoryStore) Subscribe(ctx context.Context, peerID peer.ID, last *cid.Cid, oneOff bool) (
	<-chan store.Entry,
	error,
) {
	ch := make(chan store.Entry)
	go func() {
		for _, entry := range i.Storage {
			ch <- store.Entry{Message: entry}
		}
		close(ch)
	}()
	return ch, nil
}

func (i *InMemoryStore) Publish(ctx context.Context, input []byte) error {
	i.Storage = append(i.Storage, input)
	return nil
}

func TestListPeers(t *testing.T) {
	assert := assert.New(t)
	store := InMemoryStore{
		Storage: [][]byte{},
	}

	newPeer1 := "peer1"
	err := TrustNewPeer(context.Background(), &store, newPeer1)
	assert.NoError(err)

	newPeer2 := "peer2"
	err = TrustNewPeer(context.Background(), &store, newPeer2)
	assert.NoError(err)

	err = RevokePeer(context.Background(), &store, newPeer1)
	assert.NoError(err)

	_, _, peer := helper.GeneratePeerID(t)

	peers, err := ListPeers(context.Background(), &store, peer.String())
	assert.NoError(err)
	assert.Equal(2, len(peers))
	assert.False(peers[newPeer1])
	assert.True(peers[newPeer2])
}
