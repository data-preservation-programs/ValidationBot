package store

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"sync"
	"time"

	"github.com/go-resty/resty/v2"
	"github.com/ipfs/go-cid"
	"github.com/ipfs/go-ipns"
	ipns_pb "github.com/ipfs/go-ipns/pb"
	"github.com/ipld/go-ipld-prime/codec/dagcbor"
	basicnode "github.com/ipld/go-ipld-prime/node/basic"
	"github.com/ipld/go-ipld-prime/node/bindnode"
	"github.com/libp2p/go-libp2p/core/crypto"
	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/pkg/errors"
	"github.com/rs/zerolog/log"
)

type W3StoreSubscriber struct {
	client *resty.Client
}

func NewW3StoreSubscriber() W3StoreSubscriber {
	client := resty.New()
	return W3StoreSubscriber{
		client: client,
	}
}

func (s W3StoreSubscriber) Subscribe(ctx context.Context, peerID peer.ID, last *cid.Cid) (<-chan Entry, error) {
	peerCid := peer.ToCid(peerID)
	entries := make(chan Entry)
	go func() {
		for {
			latest, err := getLastRecord(ctx, s.client, peerCid.String())
			if err != nil {
				log.Error().Err(err).Msg("failed to get last record")
				time.Sleep(time.Second * 60)
				continue
			}
			_, latestCid, err := cid.CidFromBytes(latest.Value)
			if err != nil {
				log.Error().Err(err).Msg("failed to get last cid")
				time.Sleep(time.Second * 60)
				continue
			}
			// Push all entries from latestCid until last
			for {
				got, err := s.client.R().SetContext(ctx).Get("https://api.web3.storage/car/" + latestCid.String())
				if err != nil {
					log.Error().Err(err).Msg("failed to get car")
					time.Sleep(time.Second * 60)
					continue
				}

				buffer := bytes.NewReader(got.Body())
				builder := basicnode.Prototype.Any.NewBuilder()
				err = dagcbor.Decode(builder, buffer)
				if err != nil {
					log.Error().Err(err).Msg("failed to decode car")
					continue
				}
				entry := bindnode.Unwrap(builder.Build()).(*Entry)
				entries <- *entry

				if entry.PreviousCid.Equals(*last) {
					break
				}
			}

			last = &latestCid
		}
	}()
	return entries, nil
}

type W3StorePublisher struct {
	client         *resty.Client
	peerId         peer.ID
	peerCid        cid.Cid
	privateKey     crypto.PrivKey
	lastCid        *cid.Cid
	lastSequence   *uint64
	initialized    bool
	initializedMux sync.Mutex
}

func NewW3StorePublisher(ctx context.Context, token string, privateKeyStr string) (*W3StorePublisher, error) {
	client := resty.New()
	client.SetAuthToken(token)

	privateKeyBytes, err := base64.StdEncoding.DecodeString(privateKeyStr)
	if err != nil {
		return nil, errors.Wrap(err, "cannot decode private key")
	}

	privateKey, err := crypto.UnmarshalPrivateKey(privateKeyBytes)
	if err != nil {
		return nil, errors.Wrap(err, "cannot unmarshal private key")
	}

	peerId, _ := peer.IDFromPrivateKey(privateKey)
	peerCid := peer.ToCid(peerId)

	w3Store := &W3StorePublisher{
		client:         client,
		peerId:         peerId,
		privateKey:     privateKey,
		peerCid:        peerCid,
		initialized:    false,
		initializedMux: sync.Mutex{},
	}

	return w3Store, nil
}

func (s *W3StorePublisher) initialize(ctx context.Context) error {
	s.initializedMux.Lock()
	defer s.initializedMux.Unlock()
	entry, err := getLastRecord(ctx, s.client, s.peerCid.String())
	if err != nil {
		return errors.Wrap(err, "failed to get last record")
	}

	s.lastSequence = entry.Sequence
	_, *s.lastCid, err = cid.CidFromBytes(entry.Value)
	if err != nil {
		return errors.Wrap(err, "failed to get last cid")
	}

	s.initialized = true

	return nil
}

func (s *W3StorePublisher) publishNewRecord(ctx context.Context, value []byte) error {
	*s.lastSequence++
	ipnsEntry, err := ipns.Create(s.privateKey, value, *s.lastSequence,
		time.Now().Add(time.Hour*time.Duration(24*365)),
		time.Hour*time.Duration(24*365))
	if err != nil {
		return errors.Wrap(err, "failed to create ipns entry")
	}

	encoded, err := ipnsEntry.Marshal()
	if err != nil {
		return errors.Wrap(err, "failed to encode ipns entry")
	}

	_, err = s.client.R().SetContext(ctx).SetBody(encoded).Post("https://name.web3.storage/name/" + s.peerCid.String())
	if err != nil {
		return errors.Wrap(err, "failed to publish new record")
	}

	return nil
}

func getLastRecord(ctx context.Context, client *resty.Client, peerCid string) (*ipns_pb.IpnsEntry, error) {
	resp, err := client.R().SetContext(ctx).Get("https://name.web3.storage/name/" + peerCid)
	if err != nil {
		return nil, errors.Wrap(err, "failed to get last cid")
	}

	type Response struct {
		value  string
		record string
	}
	response := new(Response)
	err = json.Unmarshal(resp.Body(), response)
	if err != nil {
		return nil, errors.Wrap(err, "failed to unmarshal response")
	}

	decoded, err := base64.StdEncoding.DecodeString(response.record)
	if err != nil {
		return nil, errors.Wrap(err, "failed to decode record")
	}

	record := new(ipns_pb.IpnsEntry)
	err = record.Unmarshal(decoded)
	if err != nil {
		return nil, errors.Wrap(err, "failed to unmarshal record")
	}

	return record, nil
}

func (s *W3StorePublisher) Publish(ctx context.Context, data []byte) error {
	content := &Entry{
		PreviousCid: *s.lastCid,
		Message:     data,
	}
	nodeRepresentation := bindnode.Wrap(content, nil).Representation()
	buffer := new(bytes.Buffer)
	err := dagcbor.Encode(nodeRepresentation, buffer)
	if err != nil {
		return errors.Wrap(err, "failed to encode content")
	}

	responseStr, err := s.client.R().SetContext(ctx).SetBody(buffer.Bytes()).Post("https://api.web3.storage/car")
	if err != nil {
		return errors.Wrap(err, "failed to upload content")
	}

	type Response struct {
		cid string
	}

	response := new(Response)
	err = json.Unmarshal(responseStr.Body(), response)
	if err != nil {
		return errors.Wrap(err, "failed to unmarshal response")
	}

	*s.lastCid, err = cid.Parse(response.cid)
	if err != nil {
		return errors.Wrap(err, "failed to parse cid")
	}

	return s.publishNewRecord(ctx, s.lastCid.Bytes())
}
