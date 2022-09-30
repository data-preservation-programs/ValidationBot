package task

import (
	"context"
	"testing"
	"validation-bot/test"

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
		assert.Equal(publisherPeerId, from)
		done <- true
	}()

	err = publisher.Publish(ctx, []byte("test"))
	assert.Nil(err)
	<-done
}
