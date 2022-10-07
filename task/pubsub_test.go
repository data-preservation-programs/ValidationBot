package task

import (
	"context"
	"testing"
	"time"

	"validation-bot/test"

	"github.com/libp2p/go-libp2p"
	pubsub "github.com/libp2p/go-libp2p-pubsub"
	"github.com/libp2p/go-libp2p/core/network"
	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/rs/zerolog/log"
	"github.com/stretchr/testify/assert"
)

func TestPubsubPublisherSubscriber(t *testing.T) {
	assert := assert.New(t)
	ctx := context.Background()
	publisherPrivate, _, publisherPeerId := test.GeneratePeerID(t)
	publisherPrivateKey := test.MarshalPrivateKey(t, publisherPrivate)
	publisherConfig, err := NewPubsubConfig(
		publisherPrivateKey,
		"/ip4/0.0.0.0/tcp/0",
		"test_topic",
	)
	assert.Nil(err)
	publisher, err := NewLibp2pTaskPublisher(ctx, *publisherConfig)
	assert.Nil(err)

	subscriberPrivate, _, _ := test.GeneratePeerID(t)
	subscriberPrivateKey := test.MarshalPrivateKey(t, subscriberPrivate)
	subscriberConfig, err := NewPubsubConfig(
		subscriberPrivateKey,
		"/ip4/0.0.0.0/tcp/0",
		"test_topic",
	)
	assert.Nil(err)
	subscriber, err := NewLibp2pTaskSubscriber(ctx, *subscriberConfig)
	assert.Nil(err)

	done := make(chan bool)

	go func() {
		from, data, err := subscriber.Next(ctx)
		assert.Nil(err)
		assert.Equal("test", string(data))
		assert.Equal(publisherPeerId, *from)
		done <- true
	}()

	err = publisher.connect(ctx, subscriber.AddrInfo())
	assert.Nil(err)
	err = publisher.Publish(ctx, []byte("test"))
	assert.Nil(err)
	<-done
}

func TestPubsub(t *testing.T) {
	assert := assert.New(t)
	ctx := context.Background()
	publisherPrivate, _, _ := test.GeneratePeerID(t)

	pubHost, err := libp2p.New(
		libp2p.ListenAddrStrings("/ip4/0.0.0.0/tcp/0"),
		libp2p.Identity(publisherPrivate))
	assert.Nil(err)

	pubPs, err := pubsub.NewGossipSub(ctx, pubHost)
	assert.Nil(err)

	pubTopic, err := pubPs.Join("test_topic")
	assert.Nil(err)

	subscriberPrivate, _, _ := test.GeneratePeerID(t)
	subHost, err := libp2p.New(
		libp2p.ListenAddrStrings("/ip4/0.0.0.0/tcp/0"),
		libp2p.Identity(subscriberPrivate))
	assert.Nil(err)

	subPs, err := pubsub.NewGossipSub(ctx, subHost)
	assert.Nil(err)

	subTopic, err := subPs.Join("test_topic")
	assert.Nil(err)

	subSubscription, err := subTopic.Subscribe()
	assert.Nil(err)

	subAddrInfo := peer.AddrInfo{
		ID:    subHost.ID(),
		Addrs: subHost.Addrs(),
	}

	done := make(chan bool)

	go func() {
		msg, err := subSubscription.Next(ctx)
		assert.Nil(err)
		assert.Equal("test", string(msg.Data))
		done <- true
	}()

	err = pubHost.Connect(network.WithForceDirectDial(ctx, "test"), subAddrInfo)
	assert.Nil(err)
	time.Sleep(5 * time.Second)
	err = pubHost.Connect(network.WithForceDirectDial(ctx, "test"), subAddrInfo)
	err = pubTopic.Publish(ctx, []byte("test")) //, pubsub.WithReadiness(pubsub.MinTopicSize(0)))
	time.Sleep(5 * time.Second)
	err = pubHost.Connect(network.WithForceDirectDial(ctx, "test"), subAddrInfo)
	err = pubTopic.Publish(ctx, []byte("test")) //, pubsub.WithReadiness(pubsub.MinTopicSize(0)))
	log.Info().Msg("published")
	assert.Nil(err)
	<-done
}
