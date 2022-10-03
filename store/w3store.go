package store

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"strings"
	"sync"
	"time"

	"github.com/go-resty/resty/v2"
	"github.com/ipfs/go-cid"
	"github.com/ipfs/go-ipns"
	ipns_pb "github.com/ipfs/go-ipns/pb"
	carv2 "github.com/ipld/go-car/v2"
	"github.com/ipld/go-car/v2/blockstore"
	"github.com/ipld/go-ipld-prime/codec/dagcbor"
	"github.com/ipld/go-ipld-prime/datamodel"
	"github.com/ipld/go-ipld-prime/fluent/qp"
	cidlink "github.com/ipld/go-ipld-prime/linking/cid"
	basicnode "github.com/ipld/go-ipld-prime/node/basic"
	"github.com/libp2p/go-libp2p/core/crypto"
	"github.com/libp2p/go-libp2p/core/peer"
	mbase "github.com/multiformats/go-multibase"
	"github.com/pkg/errors"
	"github.com/rs/zerolog/log"
)

type W3StoreSubscriber struct {
	client        *resty.Client
	retryInterval time.Duration
}

func NewW3StoreSubscriber(retryInterval time.Duration) W3StoreSubscriber {
	client := resty.New()
	return W3StoreSubscriber{
		client:        client,
		retryInterval: retryInterval,
	}
}

func (s W3StoreSubscriber) Subscribe(ctx context.Context, peerID peer.ID, last cid.Cid) (<-chan Entry, error) {
	peerCid := peer.ToCid(peerID)
	entries := make(chan Entry)
	go func() {
		for {
			peerStr, err := mbase.Encode(mbase.Base36, peerCid.Bytes())
			if err != nil {
				log.Error().Err(err).Msg("cannot encode peer cid")
				return
			}
			latest, err := getLastRecord(ctx, s.client, peerStr)
			if err != nil {
				log.Error().Err(err).Msg("failed to get last record")
				time.Sleep(s.retryInterval)
				continue
			}
			latestCid, err := cid.Decode(string(latest.Value))
			if err != nil {
				log.Error().Err(err).Msg("failed to decode last cid")
				time.Sleep(s.retryInterval)
				continue
			}
			// Push all entries from latestCid until last
			downloaded, err := s.downloadChainedEntriesUntil(ctx, latestCid)
			reverse(downloaded)
			if err != nil {
				log.Error().Err(err).Msg("failed to download chained entries")
				time.Sleep(s.retryInterval)
				continue
			}

			for _, entry := range downloaded {
				entries <- entry
				last = entry.Previous
			}
		}
	}()
	return entries, nil
}

func (s W3StoreSubscriber) downloadChainedEntries(ctx context.Context, from *cid.Cid, to cid.Cid) ([]Entry, error) {
	entries := make([]Entry, 0)
	for {
		toStr := to.String()
		got, err := s.client.R().SetContext(ctx).Get("https://api.web3.storage/car/" + toStr)
		if err != nil {
			log.Error().Err(err).Msg("failed to get car")
			time.Sleep(s.retryInterval)
			continue
		}

		car, err := carv2.NewReader(bytes.NewReader(got.Body()))

		if err != nil {
			return nil, errors.Wrap(err, "cannot read car")
		}

		dataReader, err := car.DataReader()
		if err != nil {
			return nil, errors.Wrap(err, "cannot get data reader")
		}

		store, err := blockstore.NewReadOnly(dataReader, nil,
			blockstore.UseWholeCIDs(true),
			carv2.ZeroLengthSectionAsEOF(true))
		if err != nil {
			return nil, errors.Wrap(err, "cannot create blockstore")
		}

		blk, err := store.Get(ctx, to)
		if err != nil {
			return nil, errors.Wrap(err, "cannot get block")
		}

		buffer := bytes.NewBuffer(blk.RawData())
		builder := basicnode.Prototype.List.NewBuilder()
		err = dagcbor.Decode(builder, buffer)
		if err != nil {
			return nil, errors.Wrap(err, "cannot decode block")
		}

		node := builder.Build()
		message, err := node.LookupByIndex(0)
		if err != nil {
			return nil, errors.Wrap(err, "cannot get message")
		}

		previousNode, err := node.LookupByIndex(1)
		if err != nil {
			return nil, errors.Wrap(err, "cannot get previous node")
		}

		entryBytes, err := message.AsBytes()
		if err != nil {
			return nil, errors.Wrap(err, "cannot get entry bytes")
		}

		previousCid := new(cid.Cid)

		if previousNode.Kind() == datamodel.Kind_Link {
			previousLink, err := previousNode.AsLink()
			if err != nil {
				log.Error().Err(err).Msg("failed to get previous link")
				break
			}
			c := previousLink.(cidlink.Link).Cid
			previousCid = &c
		}

		entries = append(entries, Entry{
			Message:  entryBytes,
			Previous: previousCid,
		})

		if previousCid == nil || (from != nil && previousCid.Equals(*from)) {
			break
		}
	}
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

func NewW3StorePublisher(token string, privateKeyStr string) (*W3StorePublisher, error) {
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
	if s.initialized {
		return nil
	}

	peerStr, err := mbase.Encode(mbase.Base36, s.peerCid.Bytes())
	if err != nil {
		return errors.Wrap(err, "cannot encode peer cid")
	}
	entry, err := getLastRecord(ctx, s.client, peerStr)
	if err != nil {
		return errors.Wrap(err, "failed to get last record")
	}

	if entry != nil {
		s.lastSequence = entry.Sequence
		lastCid, err := cid.Decode(string(entry.Value))
		s.lastCid = &lastCid
		if err != nil {
			return errors.Wrap(err, "failed to get last cid")
		}
	}

	s.initialized = true

	return nil
}

func (s *W3StorePublisher) publishNewName(ctx context.Context, value cid.Cid) error {
	var sequence uint64
	if s.lastSequence != nil {
		sequence = *s.lastSequence + 1
	}
	ipnsEntry, err := ipns.Create(s.privateKey, []byte(value.String()), sequence,
		time.Now().Add(time.Hour*time.Duration(24*365)),
		time.Hour*time.Duration(24*365))
	if err != nil {
		return errors.Wrap(err, "failed to create ipns entry")
	}

	encoded, err := ipnsEntry.Marshal()
	if err != nil {
		return errors.Wrap(err, "failed to encode ipns entry")
	}

	peerStr, err := mbase.Encode(mbase.Base36, s.peerCid.Bytes())
	if err != nil {
		return errors.Wrap(err, "cannot encode peer cid")
	}
	resp, err := s.client.R().SetContext(ctx).
		SetBody(base64.StdEncoding.EncodeToString(encoded)).
		Post("https://name.web3.storage/name/" + peerStr)
	if err != nil {
		return errors.Wrap(err, "failed to publish new record")
	}

	if resp.StatusCode() != 202 {
		return errors.Errorf("failed to publish new record: %d - %s",
			resp.StatusCode(), resp.Body())
	}
	if s.lastSequence != nil {
		*s.lastSequence++
	} else {
		s.lastSequence = &sequence
	}
	return nil
}

func getLastRecord(ctx context.Context, client *resty.Client, peerCid string) (*ipns_pb.IpnsEntry, error) {
	resp, err := client.R().SetContext(ctx).Get("https://name.web3.storage/name/" + peerCid)
	if err != nil {
		return nil, errors.Wrap(err, "failed to get last cid")
	}

	if resp.StatusCode() != 200 {
		if strings.Contains(string(resp.Body()), "not found") {
			return nil, nil
		}
		return nil, errors.Errorf("failed to get last cid: %d - %s",
			resp.StatusCode(), resp.Body())
	}

	type Response struct {
		Value  string `json:"value"`
		Record string `json:"record"`
	}

	response := new(Response)
	err = json.Unmarshal(resp.Body(), response)
	if err != nil {
		return nil, errors.Wrap(err, "failed to unmarshal response")
	}

	decoded, err := base64.StdEncoding.DecodeString(response.Record)
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
	node, err := qp.BuildList(basicnode.Prototype.Any, 2, func(la datamodel.ListAssembler) {
		qp.ListEntry(la, qp.Bytes(data))
		if s.lastCid != nil {
			qp.ListEntry(la, qp.Link(cidlink.Link{Cid: *s.lastCid}))
		} else {
			qp.ListEntry(la, qp.Null())
		}
	})
	buffer := new(bytes.Buffer)
	err = dagcbor.Encode(node, buffer)
	if err != nil {
		return errors.Wrap(err, "failed to encode content")
	}

	responseStr, err := s.client.R().SetContext(ctx).SetBody(buffer.Bytes()).
		Post("https://api.web3.storage/upload")
	if err != nil {
		return errors.Wrap(err, "failed to upload content")
	}

	if responseStr.StatusCode() != 200 {
		return errors.Errorf("failed to upload content: %d - %s",
			responseStr.StatusCode(), responseStr.Body())
	}

	type Response struct {
		Cid string `json:"cid"`
	}

	response := new(Response)
	err = json.Unmarshal(responseStr.Body(), response)
	if err != nil {
		return errors.Wrap(err, "failed to unmarshal response")
	}

	lastCid, err := cid.Parse(response.Cid)
	if err != nil {
		return errors.Wrap(err, "failed to parse cid")
	}
	s.lastCid = &lastCid

	return s.publishNewName(ctx, lastCid)
}
