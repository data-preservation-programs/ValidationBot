package task

import (
	"context"
	"encoding/base64"
	"time"

	"github.com/libp2p/go-libp2p"
	pubsub "github.com/libp2p/go-libp2p-pubsub"
	"github.com/libp2p/go-libp2p/core/crypto"
	"github.com/libp2p/go-libp2p/core/host"
	"github.com/libp2p/go-libp2p/core/network"
	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/pkg/errors"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
	"github.com/stretchr/testify/mock"
	"golang.org/x/exp/slices"
)

type Publisher interface {
	Publish(ctx context.Context, task []byte) error
}

type Subscriber interface {
	Next(ctx context.Context) (*peer.ID, []byte, error)
}

type Libp2pTaskSubscriber struct {
	addrInfo     peer.AddrInfo
	subscription *pubsub.Subscription
	log          zerolog.Logger
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
	log := log.With().Str("role", "task_subscriber").Logger()
	log.Info().Str("listen_addr", config.ListenAddr).Msg("creating new libp2p host")
	host, err := libp2p.New(
		libp2p.ListenAddrStrings(config.ListenAddr),
		libp2p.Identity(config.PrivateKey))
	if err != nil {
		return nil, errors.Wrap(err, "cannot create new libp2p host")
	}

	log.Info().Str("topic_name", config.TopicName).Msg("subscribing to pubsub topic")
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

	log.Info().Str("addr", addrInfo.String()).Msg("listening on")
	return &Libp2pTaskSubscriber{
		subscription: subscription,
		addrInfo:     addrInfo,
		log:          log,
	}, nil
}

func (s Libp2pTaskSubscriber) Next(ctx context.Context) (*peer.ID, []byte, error) {
	log := s.log
	log.Info().Msg("waiting for next message")
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
	log    zerolog.Logger
}

func (l Libp2pTaskPublisher) connect(ctx context.Context, addr peer.AddrInfo) error {
	// This will wait until the peer has been fully connected to the same topic
	retry := 0
	for {
		log.Info().Str("addr", addr.String()).Int("retry", retry).Msg("connecting to peer")
		err := l.libp2p.Connect(network.WithForceDirectDial(ctx, "test"), addr)
		if err != nil {
			return errors.Wrap(err, "cannot connect to peer")
		}
		if slices.Contains(l.topic.ListPeers(), addr.ID) {
			log.Info().Str("addr", addr.String()).Msg("peer connected")
			return nil
		}

		select {
		case <-ctx.Done():
			return errors.Wrap(ctx.Err(), "context is done")
		case <-time.After(500 * time.Millisecond):
			retry++
			if retry > 10 {
				return errors.New("cannot connect to peer")
			}
			continue
		}
	}
}

func (l Libp2pTaskPublisher) Publish(ctx context.Context, task []byte) error {
	log.Info().Bytes("task", task).Msg("publishing message")
	return l.topic.Publish(ctx, task)
}

func NewLibp2pTaskPublisher(ctx context.Context, config PubsubConfig) (*Libp2pTaskPublisher, error) {
	log := log.With().Str("role", "task_publisher").Logger()
	log.Info().Str("listen_addr", config.ListenAddr).Msg("creating new libp2p host")
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

	log.Info().Str("topic_name", config.TopicName).Msg("joining pubsub topic")
	topic, err := ps.Join(config.TopicName)
	if err != nil {
		return nil, errors.Wrap(err, "cannot join TopicName")
	}

	addrInfo := peer.AddrInfo{
		ID:    host.ID(),
		Addrs: host.Addrs(),
	}

	log.Info().Str("addr", addrInfo.String()).Msg("listening on")

	return &Libp2pTaskPublisher{topic: topic, libp2p: host, log: log}, nil
}

type MockPublisher struct {
	mock.Mock
}

func (m *MockPublisher) Publish(ctx context.Context, task []byte) error {
	args := m.Called(ctx, task)
	return args.Error(0)
}

type MockSubscriber struct {
	mock.Mock
}

func (m *MockSubscriber) Next(ctx context.Context) (*peer.ID, []byte, error) {
	args := m.Called(ctx)
	return args.Get(0).(*peer.ID), args.Get(1).([]byte), args.Error(2)
}
