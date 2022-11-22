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

	_, _, trustor := helper.GeneratePeerID(t)
	_, _, trustee1 := helper.GeneratePeerID(t)
	err := ModifyPeers(context.Background(), &store, &store, Create, trustor, []peer.ID{trustee1}, time.Second)
	assert.NoError(err)
	peers, err := ListPeers(context.Background(), &store, trustor)
	assert.Equal([]string{trustee1.String()}, peers)

	_, _, trustee2 := helper.GeneratePeerID(t)
	err = ModifyPeers(context.Background(), &store, &store, Create, trustor, []peer.ID{trustee2}, time.Second)
	assert.NoError(err)
	peers, err = ListPeers(context.Background(), &store, trustor)
	assert.Equal(2, len(peers))

	err = ModifyPeers(context.Background(), &store, &store, Revoke, trustor, []peer.ID{trustee2}, time.Second)
	assert.NoError(err)
	peers, err = ListPeers(context.Background(), &store, trustor)
	assert.Equal([]string{trustee1.String()}, peers)

	err = ModifyPeers(context.Background(), &store, &store, Reset, trustor, []peer.ID{trustee2}, time.Second)
	assert.NoError(err)
	peers, err = ListPeers(context.Background(), &store, trustor)
	assert.Equal([]string{trustee2.String()}, peers)
}

func TestManager_Start(t *testing.T) {
	assert := assert.New(t)
	store := store.InMemoryStore{
		Storage: [][]byte{},
	}

	_, _, trustor := helper.GeneratePeerID(t)
	_, _, newPeer1 := helper.GeneratePeerID(t)
	_, _, trustee1 := helper.GeneratePeerID(t)
	err := ModifyPeers(context.Background(), &store, &store, Create, trustor, []peer.ID{trustee1}, time.Second)
	assert.NoError(err)

	manager := NewManager([]peer.ID{trustor}, &store, time.Second, time.Second)
	ctx := context.Background()
	assert.False(manager.IsTrusted(newPeer1))
	manager.Start(ctx)
	time.Sleep(time.Second)
	assert.True(manager.IsTrusted(trustee1))

	err = ModifyPeers(context.Background(), &store, &store, Revoke, trustor, []peer.ID{trustee1}, time.Second)
	assert.NoError(err)
	time.Sleep(2 * time.Second)
	assert.False(manager.IsTrusted(trustee1))
}
