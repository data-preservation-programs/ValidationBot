package test

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"strconv"
	"testing"
	"time"

	"github.com/libp2p/go-libp2p"
	pubsub "github.com/libp2p/go-libp2p-pubsub"
	"github.com/libp2p/go-libp2p/core/crypto"
	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/multiformats/go-multiaddr"
	"github.com/rs/zerolog/log"
	"github.com/stretchr/testify/assert"
)

func GeneratePeerID(t *testing.T) (crypto.PrivKey, crypto.PubKey, peer.ID) {
	assert := assert.New(t)
	private, public, err := crypto.GenerateEd25519Key(rand.Reader)
	assert.Nil(err)
	peerID, err := peer.IDFromPublicKey(public)
	assert.Nil(err)
	log.Info().Str("peerID", peerID.String()).Msg("peerID created")
	return private, public, peerID
}

func MarshalPrivateKey(t *testing.T, private crypto.PrivKey) string {
	assert := assert.New(t)
	bytes, err := crypto.MarshalPrivateKey(private)
	assert.Nil(err)
	return base64.StdEncoding.EncodeToString(bytes)
}

func PublishTask(ctx context.Context, t *testing.T, pubPort int,
	subPort int, subPeerID peer.ID, topicName string, task []byte,
) {
	assert := assert.New(t)
	private, _, _ := GeneratePeerID(t)
	host, err := libp2p.New(
		libp2p.ListenAddrStrings("/ip4/0.0.0.0/tcp/"+strconv.Itoa(pubPort)),
		libp2p.Identity(private))
	assert.Nil(err)
	m, err := multiaddr.NewMultiaddr("/ip4/0.0.0.0/tcp/" + strconv.Itoa(subPort))
	assert.Nil(err)
	err = host.Connect(ctx, peer.AddrInfo{
		ID:    subPeerID,
		Addrs: []multiaddr.Multiaddr{m},
	})
	log.Info().Msgf("Connected to %s", subPeerID)
	assert.Nil(err)
	ps, err := pubsub.NewGossipSub(ctx, host)
	assert.Nil(err)
	topic, err := ps.Join(topicName)
	assert.Nil(err)
	// Wait until there are at least one peer
	for {
		if len(topic.ListPeers()) > 0 {
			break
		}
		time.Sleep(time.Millisecond * 100)
	}
	err = topic.Publish(ctx, task)
	log.Info().Msgf("Published task %s", task)
	assert.Nil(err)
}
