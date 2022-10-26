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
	store, err := NewW3StorePublisher(context.Background(), config)
	assert.Nil(err)
	assert.NotNil(store)
	return store
}

func TestW3StorePublisher_Initialize(t *testing.T) {
	assert := assert.New(t)
	context := context.Background()
	store := getStore(t)
	err := store.initialize(context)
	assert.Nil(err)
	err = store.initialize(context)
	assert.Nil(err)
}

func TestW3StorePublisher_publishNewRecordTwice(t *testing.T) {
	assert := assert.New(t)
	context := context.Background()
	store := getStore(t)
	err := store.publishNewName(context, cid.NewCidV1(cid.Raw, []byte("test1")))
	assert.Nil(err)
	err = store.publishNewName(context, cid.NewCidV1(cid.Raw, []byte("test2")))
	assert.Nil(err)
}

func TestW3StorePublisher_publishNewRecordAndInitialize(t *testing.T) {
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

func TestW3StoreSubscriber_downloadChainedEntries(t *testing.T) {
	assert := assert.New(t)
	ctx := context.Background()
	config := W3StoreSubscriberConfig{
		RetryInterval: time.Second,
		PollInterval:  time.Minute,
		RetryWait:     time.Second,
		RetryWaitMax:  time.Minute,
		RetryCount:    3,
	}
	subscriber := NewW3StoreSubscriber(config)
	entries, err := subscriber.downloadChainedEntries(ctx, nil, cid.MustParse("bafkreic2omcnent2hmq4jvw3ajml4kceweoyyggno5qkvqd7cz53q6qbjq"))
	assert.Nil(err)
	assert.Equal(3, len(entries))
	assert.Equal(string(entries[0].Message), "test3")
	assert.Equal(entries[0].CID.String(), "bafkreic2omcnent2hmq4jvw3ajml4kceweoyyggno5qkvqd7cz53q6qbjq")
	assert.Equal(string(entries[1].Message), "test2")
	assert.Equal(entries[1].CID.String(), "bafkreiawxfcprqxgv5rebdri465gw3bk6gcqy5iwlfstckr37sn57kh3bi")
	assert.Equal(string(entries[2].Message), "test1")
	assert.Equal(entries[2].CID.String(), "bafkreig4bdyaaedbcqy7ysylkbwkomo43aax223btxefxfcal4aiz6iw6e")
}

func TestW3StoreSubscriber_downloadChainedEntries_FromNotNil(t *testing.T) {
	assert := assert.New(t)
	ctx := context.Background()
	config := W3StoreSubscriberConfig{
		RetryInterval: time.Second,
		PollInterval:  time.Minute,
		RetryWait:     time.Second,
		RetryWaitMax:  time.Minute,
		RetryCount:    3,
	}
	subscriber := NewW3StoreSubscriber(config)
	from := cid.MustParse("bafkreiawxfcprqxgv5rebdri465gw3bk6gcqy5iwlfstckr37sn57kh3bi")
	entries, err := subscriber.downloadChainedEntries(ctx, &from, cid.MustParse("bafkreic2omcnent2hmq4jvw3ajml4kceweoyyggno5qkvqd7cz53q6qbjq"))
	assert.Nil(err)
	assert.Equal(1, len(entries))
	assert.Equal(string(entries[0].Message), "test3")
}

func TestW3StoreSubscriber_downloadChainedEntries_FromLatest(t *testing.T) {
	assert := assert.New(t)
	ctx := context.Background()
	config := W3StoreSubscriberConfig{
		RetryInterval: time.Second,
		PollInterval:  time.Minute,
		RetryWait:     time.Second,
		RetryWaitMax:  time.Minute,
		RetryCount:    3,
	}
	subscriber := NewW3StoreSubscriber(config)
	from := cid.MustParse("bafkreic2omcnent2hmq4jvw3ajml4kceweoyyggno5qkvqd7cz53q6qbjq")
	entries, err := subscriber.downloadChainedEntries(ctx, &from, cid.MustParse("bafkreic2omcnent2hmq4jvw3ajml4kceweoyyggno5qkvqd7cz53q6qbjq"))
	assert.Nil(err)
	assert.Equal(0, len(entries))
}

func TestW3StorePublisher_PublishAndSubscribe(t *testing.T) {
	assert := assert.New(t)
	ctx := context.Background()
	store := getStore(t)
	err := store.initialize(ctx)
	assert.Nil(err)
	err = store.Publish(ctx, []byte("test1"))
	assert.Nil(err)
	assert.NotNil(store.lastCid)
	assert.Equal(uint64(0), *store.lastSequence)
	err = store.Publish(ctx, []byte("test2"))
	assert.Nil(err)
	assert.NotNil(store.lastCid)
	assert.Equal(uint64(1), *store.lastSequence)
	err = store.Publish(ctx, []byte("test3"))
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
	time.Sleep(time.Second * 5)
	subscriber := NewW3StoreSubscriber(config)
	entryChan, err := subscriber.Subscribe(ctx, store.peerID, nil)
	for _, expected := range []string{"test1", "test2", "test3"} {
		select {
		case entry := <-entryChan:
			assert.Equal(expected, string(entry.Message))
		case <-time.After(5 * time.Second):
			assert.FailNow("timeout")
			break
		}
	}
}
