package task

import (
	"context"
	"encoding/base64"
	"time"

	"github.com/libp2p/go-libp2p"
	pubsub "github.com/libp2p/go-libp2p-pubsub"
	"github.com/libp2p/go-libp2p/core/crypto"
	"github.com/libp2p/go-libp2p/core/host"
	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/pkg/errors"
	"github.com/rs/zerolog/log"
	"golang.org/x/exp/slices"
)

type Publisher interface {
	Publish(ctx context.Context, task []byte) error
	Connect(ctx context.Context, addr peer.AddrInfo) error
}

type Subscriber interface {
	Next(ctx context.Context) (*peer.ID, []byte, error)
	AddrInfo() peer.AddrInfo
}

type Libp2pTaskSubscriber struct {
	addrInfo     peer.AddrInfo
	subscription *pubsub.Subscription
}

func (s Libp2pTaskSubscriber) AddrInfo() peer.AddrInfo {
	return s.addrInfo
}

type PubsubConfig struct {
	PrivateKey crypto.PrivKey
	PeerID     peer.ID
	ListenAddr string
	TopicName  string
}

func NewPubsubConfig(privateKeyStr string, listenAddr string, topicName string) (*PubsubConfig, error) {
	privateKeyBytes, err := base64.StdEncoding.DecodeString(privateKeyStr)
	if err != nil {
		return nil, errors.Wrap(err, "cannot decode private key")
	}

	privateKey, err := crypto.UnmarshalPrivateKey(privateKeyBytes)
	if err != nil {
		return nil, errors.Wrap(err, "cannot unmarshal private key")
	}

	peerID, err := peer.IDFromPrivateKey(privateKey)

	pubsubConfig := PubsubConfig{
		PrivateKey: privateKey,
		PeerID:     peerID,
		ListenAddr: listenAddr,
		TopicName:  topicName,
	}

	return &pubsubConfig, nil
}

func NewLibp2pTaskSubscriber(ctx context.Context, config PubsubConfig) (*Libp2pTaskSubscriber, error) {
	host, err := libp2p.New(
		libp2p.ListenAddrStrings(config.ListenAddr),
		libp2p.Identity(config.PrivateKey))
	if err != nil {
		return nil, errors.Wrap(err, "cannot create new libp2p host")
	}

	ps, err := pubsub.NewGossipSub(ctx, host)
	if err != nil {
		return nil, errors.Wrap(err, "cannot create new gossip sub")
	}

	topic, err := ps.Join(config.TopicName)
	if err != nil {
		return nil, errors.Wrap(err, "cannot join TopicName")
	}

	subscription, err := topic.Subscribe()
	if err != nil {
		return nil, errors.Wrap(err, "cannot subscribe to TopicName")
	}

	addrInfo := peer.AddrInfo{
		ID:    host.ID(),
		Addrs: host.Addrs(),
	}

	return &Libp2pTaskSubscriber{subscription: subscription, addrInfo: addrInfo}, nil
}

func (s Libp2pTaskSubscriber) Next(ctx context.Context) (*peer.ID, []byte, error) {
	msg, err := s.subscription.Next(ctx)
	if err != nil {
		return nil, nil, errors.Wrap(err, "cannot get next message")
	}

	log.Info().Str("from", msg.GetFrom().String()).Str("data", string(msg.Data)).Msg("received message")

	return &msg.ReceivedFrom, msg.Data, nil
}

type Libp2pTaskPublisher struct {
	topic  *pubsub.Topic
	libp2p host.Host
}

func (l Libp2pTaskPublisher) Connect(ctx context.Context, addr peer.AddrInfo) error {
	err := l.libp2p.Connect(ctx, addr)
	if err != nil {
		return errors.Wrap(err, "cannot connect to peer")
	}

	// This will wait until the peer has been fully connected to the same topic
	for {
		if slices.Contains(l.topic.ListPeers(), addr.ID) {
			return nil
		}

		select {
		case <-ctx.Done():
			return errors.Wrap(ctx.Err(), "context is done")
		case <-time.After(100 * time.Millisecond):
			continue
		}
	}
}

func (l Libp2pTaskPublisher) Publish(ctx context.Context, task []byte) error {
	return l.topic.Publish(ctx, task)
}

func NewLibp2pTaskPublisher(ctx context.Context, config PubsubConfig) (*Libp2pTaskPublisher, error) {
	host, err := libp2p.New(
		libp2p.ListenAddrStrings(config.ListenAddr),
		libp2p.Identity(config.PrivateKey))
	if err != nil {
		return nil, errors.Wrap(err, "cannot create new libp2p host")
	}

	ps, err := pubsub.NewGossipSub(ctx, host)
	if err != nil {
		return nil, errors.Wrap(err, "cannot create new gossip sub")
	}

	topic, err := ps.Join(config.TopicName)
	if err != nil {
		return nil, errors.Wrap(err, "cannot join TopicName")
	}

	return &Libp2pTaskPublisher{topic: topic, libp2p: host}, nil
}
