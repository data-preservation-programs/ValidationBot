package trust

import (
	"context"
	"testing"
	"time"

	"validation-bot/helper"
	"validation-bot/store"

	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/stretchr/testify/assert"
)

func TestListPeers(t *testing.T) {
	assert := assert.New(t)
	store := store.InMemoryStore{
		Storage: [][]byte{},
	}

	_, _, newPeer1 := helper.GeneratePeerID(t)
	err := AddNewPeer(context.Background(), &store, newPeer1)
	assert.NoError(err)

	_, _, newPeer2 := helper.GeneratePeerID(t)
	err = AddNewPeer(context.Background(), &store, newPeer2)
	assert.NoError(err)

	err = RevokePeer(context.Background(), &store, newPeer1)
	assert.NoError(err)

	_, _, peer := helper.GeneratePeerID(t)

	peers, err := ListPeers(context.Background(), &store, peer)
	assert.NoError(err)
	assert.Equal(2, len(peers))
	assert.False(peers[newPeer1])
	assert.True(peers[newPeer2])
}

func TestManager_Start(t *testing.T) {
	assert := assert.New(t)
	store := store.InMemoryStore{
		Storage: [][]byte{},
	}

	_, _, trustor := helper.GeneratePeerID(t)
	_, _, newPeer1 := helper.GeneratePeerID(t)
	err := AddNewPeer(context.Background(), &store, newPeer1)
	assert.NoError(err)

	manager := NewManager([]peer.ID{trustor}, &store)
	ctx := context.Background()
	assert.False(manager.IsTrusted(newPeer1))
	diffEmited := false
	manager.SubscribeToDiff(
		func(diff map[peer.ID]struct{}) {
			assert.Equal(1, len(diff))
			diffEmited = true
		},
	)
	manager.Start(ctx)
	time.Sleep(time.Second)
	assert.True(manager.IsTrusted(newPeer1))
	assert.True(diffEmited)
}
