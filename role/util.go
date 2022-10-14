package role

import (
	"crypto/rand"
	"encoding/base64"

	"github.com/libp2p/go-libp2p"
	"github.com/libp2p/go-libp2p/core/crypto"
	"github.com/libp2p/go-libp2p/core/host"
	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/pkg/errors"
)

func NewLibp2pHost(privateKeyStr string, listenAddr string) (*host.Host, error) {
	privateKeyBytes, err := base64.StdEncoding.DecodeString(privateKeyStr)
	if err != nil {
		return nil, errors.Wrap(err, "cannot decode private key")
	}

	privateKey, err := crypto.UnmarshalPrivateKey(privateKeyBytes)
	if err != nil {
		return nil, errors.Wrap(err, "cannot unmarshal private key")
	}

	host, err := libp2p.New(
		libp2p.ListenAddrStrings(listenAddr),
		libp2p.Identity(privateKey))
	if err != nil {
		return nil, errors.Wrap(err, "cannot create new libp2p host")
	}

	return &host, nil
}

func GenerateNewPeer() (string, string, string, error) {
	private, public, err := crypto.GenerateEd25519Key(rand.Reader)
	if err != nil {
		return "", "", "", err
	}

	peerID, err := peer.IDFromPublicKey(public)
	if err != nil {
		return "", "", "", err
	}

	privateBytes, err := crypto.MarshalPrivateKey(private)
	if err != nil {
		return "", "", "", err
	}

	privateStr := base64.StdEncoding.EncodeToString(privateBytes)

	publicBytes, err := crypto.MarshalPublicKey(public)
	if err != nil {
		return "", "", "", err
	}
	publicStr := base64.StdEncoding.EncodeToString(publicBytes)
	return privateStr, publicStr, peerID.String(), nil
}
