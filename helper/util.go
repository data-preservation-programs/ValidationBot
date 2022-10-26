package helper

import (
	"crypto/rand"
	"encoding/base64"
	"testing"

	"github.com/libp2p/go-libp2p/core/crypto"
	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/rs/zerolog/log"
	"github.com/stretchr/testify/assert"
)

const PostgresConnectionString = "host=localhost port=5432 user=postgres password=postgres dbname=postgres"

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
