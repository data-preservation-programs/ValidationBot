package store

import (
	"context"
	"os"
	"testing"
	"time"

	"validation-bot/test"

	"github.com/ipfs/go-cid"
	"github.com/stretchr/testify/assert"
)

func getStore(t *testing.T) *W3StorePublisher {
	assert := assert.New(t)
	token := os.Getenv("W3S_TOKEN_TEST")
	assert.NotEmpty(token)
	privateKey, _, _ := test.GeneratePeerID(t)
	privateKeyStr := test.MarshalPrivateKey(t, privateKey)
	config := W3StorePublisherConfig{
		Token:        token,
		PrivateKey:   privateKeyStr,
		RetryWait:    time.Second * 10,
		RetryWaitMax: time.Minute,
		RetryCount:   5,
	}
	store, err := NewW3StorePublisher(config)
	assert.Nil(err)
	assert.NotNil(store)
	return store
}

func TestW3StorePublisher_Initialize(t *testing.T) {
	t.Parallel()
	assert := assert.New(t)
	context := context.Background()
	store := getStore(t)
	err := store.initialize(context)
	assert.Nil(err)
	err = store.initialize(context)
	assert.Nil(err)
}

func TestW3StorePublisher_publishNewRecordTwice(t *testing.T) {
	t.Parallel()
	assert := assert.New(t)
	context := context.Background()
	store := getStore(t)
	err := store.publishNewName(context, cid.NewCidV1(cid.Raw, []byte("test1")))
	assert.Nil(err)
	err = store.publishNewName(context, cid.NewCidV1(cid.Raw, []byte("test2")))
	assert.Nil(err)
}

func TestW3StorePublisher_publishNewRecordAndInitialize(t *testing.T) {
	t.Parallel()
	assert := assert.New(t)
	context := context.Background()
	store := getStore(t)
	testCid, err := cid.Decode("bafzaajaiaejcagb4edyjdeyp3zsewinpmgst6asomyslo7g6nd7magc42imposgv")
	assert.Nil(err)
	err = store.publishNewName(context, testCid)
	assert.Nil(err)
	err = store.initialize(context)
	assert.Nil(err)
	assert.Equal(uint64(0), *store.lastSequence)
	assert.Equal(testCid, *store.lastCid)
}

func TestW3StorePublisher_PublishAndSubscribe(t *testing.T) {
	t.Parallel()
	assert := assert.New(t)
	context := context.Background()
	store := getStore(t)
	err := store.initialize(context)
	assert.Nil(err)
	err = store.Publish(context, []byte("test1"))
	assert.Nil(err)
	assert.NotNil(store.lastCid)
	assert.Equal(uint64(0), *store.lastSequence)
	err = store.Publish(context, []byte("test2"))
	assert.Nil(err)
	assert.NotNil(store.lastCid)
	assert.Equal(uint64(1), *store.lastSequence)
	err = store.Publish(context, []byte("test3"))
	assert.Nil(err)
	assert.NotNil(store.lastCid)
	assert.Equal(uint64(2), *store.lastSequence)

	config := W3StoreSubscriberConfig{
		RetryInterval: time.Second,
		PollInterval:  time.Minute,
		RetryWait:     time.Second,
		RetryWaitMax:  time.Minute,
		RetryCount:    3,
	}
	subscriber := NewW3StoreSubscriber(config)
	entryChan, err := subscriber.Subscribe(context, store.peerID, nil)
	for _, expected := range []string{"test1", "test2", "test3"} {
		select {
		case entry := <-entryChan:
			assert.Equal(expected, string(entry.Message))
		case <-time.After(5 * time.Second):
			assert.Fail("timeout")
		}
	}
}
