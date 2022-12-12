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
)

type PublisherSubscriber interface {
	Publish(ctx context.Context, task []byte) error
	Next(ctx context.Context) (*peer.ID, []byte, error)
}

type Libp2pPublisherSubscriber struct {
	addrInfo peer.AddrInfo
	msgChan  chan *pubsub.Message
	log      zerolog.Logger
	libp2p   host.Host
	topic    *pubsub.Topic
}

func (l Libp2pPublisherSubscriber) AddrInfo() peer.AddrInfo {
	return l.addrInfo
}

func NewLibp2pPublisherSubscriber(ctx context.Context, libp2p host.Host, topicName string) (
	*Libp2pPublisherSubscriber,
	error,
) {
	log := log2.With().Str("role", "publisher_subscriber").Caller().Logger()

	discovery, err := getTopicDiscovery(ctx, libp2p)
	if err != nil {
		return nil, errors.Wrap(err, "cannot create topic discovery")
	}

	gossipSub, err := pubsub.NewGossipSub(ctx, libp2p, pubsub.WithDiscovery(discovery))
	if err != nil {
		return nil, errors.Wrap(err, "cannot create new gossip sub")
	}

	addrInfo := peer.AddrInfo{
		ID:    libp2p.ID(),
		Addrs: libp2p.Addrs(),
	}

	log.Info().Str("addr", addrInfo.String()).Msg("listening on")

	msgChan := make(chan *pubsub.Message)

	log.Info().Str("topic", topicName).Msg("subscribing to topic")

	topic, err := gossipSub.Join(topicName)
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
			log.Debug().Str("peer", libp2p.ID().String()).
				Int("peers", len(topic.ListPeers())).
				Str("topic", topicName).
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

	return &Libp2pPublisherSubscriber{
		libp2p:   libp2p,
		msgChan:  msgChan,
		addrInfo: addrInfo,
		log:      log,
		topic:    topic,
	}, nil
}

func (l Libp2pPublisherSubscriber) Next(ctx context.Context) (*peer.ID, []byte, error) {
	select {
	case <-ctx.Done():
		return nil, nil, errors.Wrap(ctx.Err(), "context is done")
	case msg := <-l.msgChan:
		from := msg.GetFrom()
		l.log.Info().Str("from", from.String()).Bytes("data", msg.Data).Msg("received message")
		return &from, msg.Data, nil
	}
}

func (l Libp2pPublisherSubscriber) Publish(ctx context.Context, task []byte) error {
	l.log.Debug().Bytes("task", task).Caller().Msg("publishing message")
	return errors.Wrap(l.topic.Publish(ctx, task), "cannot publish message")
}

func getTopicDiscovery(ctx context.Context, h host.Host) (discovery.Discovery, error) {
	kdht, err := initDHT(ctx, h)
	if err != nil {
		return nil, errors.Wrap(err, "cannot init dht")
	}

	baseDisc := routing.NewRoutingDiscovery(kdht)
	//nolint:gosec
	rng := rand.New(rand.NewSource(rand.Int63()))
	//nolint:gomnd
	discovery, err := backoff.NewBackoffDiscovery(
		baseDisc,
		backoff.NewExponentialBackoff(time.Second, time.Minute, backoff.FullJitter, time.Second, 2, time.Second, rng),
	)
	if err != nil {
		return nil, errors.Wrap(err, "cannot create backoff discovery")
	}

	return discovery, nil
}

func initDHT(ctx context.Context, libp2p host.Host) (*dht.IpfsDHT, error) {
	log := log2.With().Str("role", "dht").Caller().Logger()
	log.Info().Msg("creating new DHT")

	kdht, err := dht.New(ctx, libp2p)
	if err != nil {
		return nil, errors.Wrap(err, "cannot create new dht")
	}

	err = kdht.Bootstrap(ctx)
	if err != nil {
		return nil, errors.Wrap(err, "cannot bootstrap dht")
	}

	log.Info().Msg("waiting for DHT to be ready")

	var waitGroup sync.WaitGroup

	for _, peerAddr := range dht.DefaultBootstrapPeers {
		peerInfo, _ := peer.AddrInfoFromP2pAddr(peerAddr)

		waitGroup.Add(1)

		go func() {
			defer waitGroup.Done()
			log.Debug().Str("peer", peerInfo.String()).Msg("connecting to bootstrap peer")

			if err := libp2p.Connect(ctx, *peerInfo); err != nil {
				log.Warn().Err(err).Msg("cannot connect to bootstrap peer")
			}
		}()
	}

	waitGroup.Wait()
	return kdht, nil
}
