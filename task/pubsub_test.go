package task

import (
	"context"
	"testing"
	"time"
	"validation-bot/role"
	"validation-bot/test"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
)

func TestPubsubPublisherSubscriber(t *testing.T) {
	topicName := "topic-" + uuid.New().String()

	assert := assert.New(t)
	ctx := context.Background()
	publisherPrivate, _, _ := test.GeneratePeerID(t)
	publisherPrivateKey := test.MarshalPrivateKey(t, publisherPrivate)
	pubHost, err := role.NewLibp2pHost(publisherPrivateKey, "/ip4/0.0.0.0/tcp/8201")
	assert.Nil(err)
	assert.NotNil(pubHost)
	publisher, err := NewLibp2pTaskPublisher(ctx, *pubHost, topicName)
	assert.Nil(err)

	done := make(chan bool)

	go func() {
		subscriberPrivate, _, _ := test.GeneratePeerID(t)
		subscriberPrivateKey := test.MarshalPrivateKey(t, subscriberPrivate)
		subHost, err := role.NewLibp2pHost(subscriberPrivateKey, "/ip4/0.0.0.0/tcp/8202")
		assert.Nil(err)
		subscriber, err := NewLibp2pTaskSubscriber(ctx, *subHost, topicName)
		assert.Nil(err)
		_, data, err := subscriber.Next(ctx)
		assert.Nil(err)
		assert.Equal("hello", string(data))
		done <- true
	}()

	go func() {
		for {
			time.Sleep(1 * time.Second)
			err = publisher.Publish(ctx, []byte("hello"))
			assert.Nil(err)
		}
	}()

	select {
	case <-done:
		return
	case <-time.After(120 * time.Second):
		t.Fatal("timeout")
	}
}
