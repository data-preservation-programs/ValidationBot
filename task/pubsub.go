package task

import (
	"context"
	"math/rand"
	"sync"
	"time"

	"github.com/libp2p/go-libp2p-kad-dht"
	pubsub "github.com/libp2p/go-libp2p-pubsub"
	"github.com/libp2p/go-libp2p/core/discovery"
	"github.com/libp2p/go-libp2p/core/host"
	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/libp2p/go-libp2p/p2p/discovery/backoff"
	"github.com/libp2p/go-libp2p/p2p/discovery/routing"
	"github.com/pkg/errors"
	"github.com/rs/zerolog"
	log2 "github.com/rs/zerolog/log"
	"github.com/stretchr/testify/mock"
)

type Publisher interface {
	Publish(ctx context.Context, task []byte) error
}

type Subscriber interface {
	Next(ctx context.Context) (*peer.ID, []byte, error)
}

type Libp2pTaskSubscriber struct {
	addrInfo peer.AddrInfo
	msgChan  chan *pubsub.Message
	log      zerolog.Logger
	libp2p   host.Host
}

func (s Libp2pTaskSubscriber) AddrInfo() peer.AddrInfo {
	return s.addrInfo
}

func NewLibp2pTaskSubscriber(ctx context.Context, libp2p host.Host, topicNames []string) (*Libp2pTaskSubscriber, error) {
	log := log2.With().Str("role", "task_subscriber").Caller().Logger()

	discovery, err := getTopicDiscovery(ctx, libp2p)
	if err != nil {
		return nil, errors.Wrap(err, "cannot create topic discovery")
	}

	ps, err := pubsub.NewGossipSub(ctx, libp2p, pubsub.WithDiscovery(discovery))
	if err != nil {
		return nil, errors.Wrap(err, "cannot create new gossip sub")
	}

	addrInfo := peer.AddrInfo{
		ID:    libp2p.ID(),
		Addrs: libp2p.Addrs(),
	}

	log.Info().Str("addr", addrInfo.String()).Msg("listening on")
	msgChan := make(chan *pubsub.Message)

	for _, topicName := range topicNames {
		log.Info().Str("topic", topicName).Msg("subscribing to topic")
		topicName := topicName
		topic, err := ps.Join(topicName)
		if err != nil {
			return nil, errors.Wrap(err, "cannot join TopicName")
		}

		subscription, err := topic.Subscribe()
		if err != nil {
			return nil, errors.Wrap(err, "cannot subscribe to TopicName")
		}

		go func() {
			for {
				time.Sleep(time.Minute)
				peers := topic.ListPeers()
				log.Debug().Int("peers", len(peers)).Str("topic", topicName).
					Msg("connected to peers in the topic")
			}
		}()

		go func() {
			for {
				msg, err := subscription.Next(ctx)
				if err != nil {
					log.Error().Str("topic", topicName).Err(err).Msg("cannot get next message")
					time.Sleep(time.Minute)
					continue
				}
				msgChan <- msg
			}
		}()
	}

	return &Libp2pTaskSubscriber{
		libp2p:   libp2p,
		msgChan:  msgChan,
		addrInfo: addrInfo,
		log:      log,
	}, nil
}

func (s Libp2pTaskSubscriber) Next(ctx context.Context) (*peer.ID, []byte, error) {
	select {
	case <-ctx.Done():
		return nil, nil, ctx.Err()
	case msg := <-s.msgChan:
		s.log.Info().Str("from", msg.GetFrom().String()).Bytes("data", msg.Data).Msg("received message")
		return &msg.ReceivedFrom, msg.Data, nil
	}
}

type Libp2pTaskPublisher struct {
	topic  *pubsub.Topic
	libp2p host.Host
	log    zerolog.Logger
}

func (l Libp2pTaskPublisher) Publish(ctx context.Context, task []byte) error {
	l.log.Debug().Bytes("task", task).Caller().Msg("publishing message")
	return l.topic.Publish(ctx, task)
}

func NewLibp2pTaskPublisher(ctx context.Context, libp2p host.Host, topicName string) (*Libp2pTaskPublisher, error) {
	log := log2.With().Str("role", "task_publisher").Caller().Logger()

	discovery, err := getTopicDiscovery(ctx, libp2p)
	if err != nil {
		return nil, errors.Wrap(err, "cannot create topic discovery")
	}

	ps, err := pubsub.NewGossipSub(ctx, libp2p, pubsub.WithDiscovery(discovery))
	if err != nil {
		return nil, errors.Wrap(err, "cannot create new gossip sub")
	}

	log.Info().Str("topic_name", topicName).Msg("joining pubsub topic")
	topic, err := ps.Join(topicName)
	if err != nil {
		return nil, errors.Wrap(err, "cannot join TopicName")
	}

	addrInfo := peer.AddrInfo{
		ID:    libp2p.ID(),
		Addrs: libp2p.Addrs(),
	}

	log.Info().Str("addr", addrInfo.String()).Msg("listening on")

	go func() {
		for {
			time.Sleep(time.Minute)
			peers := topic.ListPeers()
			log.Debug().Int("peers", len(peers)).Msg("connected to peers in the topic")
		}
	}()

	return &Libp2pTaskPublisher{topic: topic, libp2p: libp2p, log: log}, nil
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

//nolint:all
func (m *MockSubscriber) Next(ctx context.Context) (*peer.ID, []byte, error) {
	args := m.Called(ctx)
	return args.Get(0).(*peer.ID), args.Get(1).([]byte), args.Error(2)
}

//nolint:ireturn
func getTopicDiscovery(ctx context.Context, h host.Host) (discovery.Discovery, error) {
	kdht, err := initDHT(ctx, h)
	if err != nil {
		return nil, errors.Wrap(err, "cannot init dht")
	}

	baseDisc := routing.NewRoutingDiscovery(kdht)
	minBackoff, maxBackoff := time.Second, time.Hour
	//nolint:gosec
	rng := rand.New(rand.NewSource(rand.Int63()))
	//nolint:gomnd
	discovery, err := backoff.NewBackoffDiscovery(
		baseDisc,
		backoff.NewExponentialBackoff(minBackoff, maxBackoff, backoff.FullJitter, time.Second, 2, time.Second, rng),
	)
	if err != nil {
		return nil, errors.Wrap(err, "cannot create backoff discovery")
	}

	return discovery, nil
}

func initDHT(ctx context.Context, h host.Host) (*dht.IpfsDHT, error) {
	log := log2.With().Str("role", "dht").Caller().Logger()
	log.Info().Msg("creating new DHT")
	kdht, err := dht.New(ctx, h)
	if err != nil {
		return nil, errors.Wrap(err, "cannot create new dht")
	}

	err = kdht.Bootstrap(ctx)
	if err != nil {
		return nil, errors.Wrap(err, "cannot bootstrap dht")
	}

	log.Info().Msg("waiting for DHT to be ready")
	var wg sync.WaitGroup
	for _, peerAddr := range dht.DefaultBootstrapPeers {
		peerInfo, _ := peer.AddrInfoFromP2pAddr(peerAddr)
		wg.Add(1)
		go func() {
			defer wg.Done()
			log.Debug().Str("peer", peerInfo.String()).Msg("connecting to bootstrap peer")
			if err := h.Connect(ctx, *peerInfo); err != nil {
				log.Warn().Err(err).Msg("cannot connect to bootstrap peer")
			}
		}()
	}

	wg.Wait()
	return kdht, nil
}
