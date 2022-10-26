package store

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"net/http"
	"strings"
	"time"

	"github.com/go-resty/resty/v2"
	"github.com/ipfs/go-cid"
	"github.com/ipfs/go-ipns"
	ipns_pb "github.com/ipfs/go-ipns/pb"
	"github.com/ipld/go-ipld-prime/codec/dagcbor"
	"github.com/ipld/go-ipld-prime/datamodel"
	"github.com/ipld/go-ipld-prime/fluent/qp"
	cidlink "github.com/ipld/go-ipld-prime/linking/cid"
	basicnode "github.com/ipld/go-ipld-prime/node/basic"
	"github.com/libp2p/go-libp2p/core/crypto"
	"github.com/libp2p/go-libp2p/core/peer"
	mbase "github.com/multiformats/go-multibase"
	"github.com/pkg/errors"
	"github.com/rs/zerolog"
	log2 "github.com/rs/zerolog/log"
)

const year = 365 * 24 * time.Hour

type W3StoreSubscriber struct {
	client        *resty.Client
	retryInterval time.Duration
	pollInterval  time.Duration
	log           zerolog.Logger
}

type W3StoreSubscriberConfig struct {
	RetryInterval time.Duration
	PollInterval  time.Duration
	RetryWait     time.Duration
	RetryWaitMax  time.Duration
	RetryCount    int
}

func NewW3StoreSubscriber(config W3StoreSubscriberConfig) W3StoreSubscriber {
	client := resty.New()
	client.SetRetryCount(config.RetryCount).SetRetryWaitTime(config.RetryWait).SetRetryMaxWaitTime(config.RetryWaitMax).
		SetRetryAfter(nil).
		AddRetryCondition(
			func(r *resty.Response, err error) bool {
				return r.StatusCode() >= 500 || r.StatusCode() == 429
			},
		)

	return W3StoreSubscriber{
		client:        client,
		retryInterval: config.RetryInterval,
		pollInterval:  config.PollInterval,
		log:           log2.With().Str("role", "w3store_subscriber").Caller().Logger(),
	}
}

func (s W3StoreSubscriber) Subscribe(ctx context.Context, peerID peer.ID, last *cid.Cid) (<-chan Entry, error) {
	log := s.log.With().Str("peer_id", peerID.String()).Logger()
	peerCid := peer.ToCid(peerID)

	peerStr, err := mbase.Encode(mbase.Base36, peerCid.Bytes())
	if err != nil {
		return nil, errors.Wrap(err, "cannot encode peer cid")
	}

	entries := make(chan Entry)

	log.Info().Interface("lastCid", last).Str("peerCid", peerStr).Msg("subscribing to w3store updates")

	go func() {
		for {
			latest, err := getLastRecord(ctx, log, s.client, peerStr)
			if err != nil {
				log.Error().Err(err).Msg("failed to get last record")
				time.Sleep(s.retryInterval)
				continue
			}

			if latest == nil {
				log.Debug().Msg("no records found")
				time.Sleep(s.pollInterval)
				continue
			}

			latestCid, err := cid.Decode(string(latest.Value))
			if err != nil {
				log.Error().Err(err).Msg("failed to decode last cid")
				time.Sleep(s.retryInterval)
				continue
			}
			// Push all entries from latestCid until last
			downloaded, err := s.downloadChainedEntries(ctx, last, latestCid)
			if err != nil {
				log.Error().Err(err).Msg("failed to download chained entries")
				time.Sleep(s.retryInterval)
				continue
			}

			log.Debug().Int("count", len(downloaded)).Msg("downloaded entries")

			for i := len(downloaded) - 1; i >= 0; i-- {
				entries <- downloaded[i]
				last = &downloaded[i].CID
			}

			time.Sleep(s.pollInterval)
		}
	}()

	return entries, nil
}

//nolint:funlen,gocognit,cyclop,varnamelen
func (s W3StoreSubscriber) downloadChainedEntries(ctx context.Context, fromExclusive *cid.Cid, to cid.Cid) (
	[]Entry,
	error,
) {
	entries := make([]Entry, 0)
	log := s.log.With().Interface("from", fromExclusive).Str("to", to.String()).Logger()

	log.Debug().Msg("downloading chained entries")

	for {
		if fromExclusive != nil && to == *fromExclusive {
			break
		}

		toStr := to.String()
		url := "https://w3s.link/ipfs/" + toStr
		log.Debug().Str("url", url).Msg("downloading ipfs content")
		got, err := s.client.R().SetContext(ctx).Get(url)
		log.Debug().Str("url", url).Dur("timeSpent", got.Time()).Str(
			"status",
			got.Status(),
		).Msg("download response received")

		if err != nil {
			return nil, errors.Wrap(err, "failed to download car")
		}

		if got.StatusCode() != http.StatusOK {
			return nil, errors.Errorf("failed to get car %d - %s", got.StatusCode(), string(got.Body()))
		}

		builder := basicnode.Prototype.List.NewBuilder()

		err = dagcbor.Decode(builder, bytes.NewReader(got.Body()))
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

		// This is the end of the chain
		if previousNode.Kind() != datamodel.Kind_Link {
			log.Debug().Interface("previous", nil).Str("cid", to.String()).Msg("retrieved a new entry")

			entries = append(
				entries, Entry{
					Message:  entryBytes,
					Previous: nil,
					CID:      to,
				},
			)
			break
		}

		previousLink, err := previousNode.AsLink()
		if err != nil {
			return nil, errors.Wrap(err, "cannot get previous link")
		}

		link, ok := previousLink.(cidlink.Link)
		if !ok {
			return nil, errors.New("cannot convert to cid link")
		}

		c := link.Cid
		if to == c {
			return nil, errors.New("previous cid is the same as current")
		}

		log.Debug().Str("previous", c.String()).Str("cid", to.String()).Msg("retrieved a new entry")

		entries = append(
			entries, Entry{
				Message:  entryBytes,
				Previous: &c,
				CID:      to,
			},
		)
		to = c
	}

	return entries, nil
}

type W3StorePublisher struct {
	client       *resty.Client
	peerID       peer.ID
	peerCid      cid.Cid
	privateKey   crypto.PrivKey
	lastCid      *cid.Cid
	lastSequence *uint64
	log          zerolog.Logger
}

type W3StorePublisherConfig struct {
	Token        string
	PrivateKey   string
	RetryWait    time.Duration
	RetryWaitMax time.Duration
	RetryCount   int
}

func NewW3StorePublisher(ctx context.Context, config W3StorePublisherConfig) (*W3StorePublisher, error) {
	client := resty.New()
	client.SetRetryCount(config.RetryCount).SetRetryWaitTime(config.RetryWait).SetRetryMaxWaitTime(config.RetryWaitMax).
		SetRetryAfter(nil).
		AddRetryCondition(
			func(r *resty.Response, err error) bool {
				return r.StatusCode() >= 500 || r.StatusCode() == 429
			},
		)
	client.SetAuthToken(config.Token)

	privateKeyBytes, err := base64.StdEncoding.DecodeString(config.PrivateKey)
	if err != nil {
		return nil, errors.Wrap(err, "cannot decode private key")
	}

	privateKey, err := crypto.UnmarshalPrivateKey(privateKeyBytes)
	if err != nil {
		return nil, errors.Wrap(err, "cannot unmarshal private key")
	}

	peerID, _ := peer.IDFromPrivateKey(privateKey)
	peerCid := peer.ToCid(peerID)

	w3Store := &W3StorePublisher{
		client:       client,
		peerID:       peerID,
		peerCid:      peerCid,
		privateKey:   privateKey,
		lastCid:      nil,
		lastSequence: nil,
		log:          log2.With().Str("role", "w3store_publisher").Caller().Logger(),
	}

	err = w3Store.initialize(ctx)
	if err != nil {
		return nil, errors.Wrap(err, "cannot initialize publisher")
	}

	return w3Store, nil
}

func (s *W3StorePublisher) initialize(ctx context.Context) error {
	peerStr, err := mbase.Encode(mbase.Base36, s.peerCid.Bytes())
	if err != nil {
		return errors.Wrap(err, "cannot encode peer cid")
	}

	entry, err := getLastRecord(ctx, s.log, s.client, peerStr)
	if err != nil {
		return errors.Wrap(err, "failed to get last record")
	}

	if entry != nil {
		s.lastSequence = entry.Sequence

		lastCid, err := cid.Decode(string(entry.Value))
		if err != nil {
			return errors.Wrap(err, "failed to get last cid")
		}

		s.lastCid = &lastCid
	}

	s.log.Info().Interface("lastSequence", s.lastSequence).
		Interface("lastCid", s.lastCid).Msg("initialized w3store publisher")

	return nil
}

func (s *W3StorePublisher) publishNewName(ctx context.Context, value cid.Cid) error {
	log := s.log.With().Str("value", value.String()).Logger()
	log.Debug().Msg("publishing new name")

	sequence := uint64(0)
	if s.lastSequence != nil {
		sequence = *s.lastSequence + 1
	}

	ipnsEntry, err := ipns.Create(
		s.privateKey, []byte(value.String()), sequence,
		time.Now().Add(year),
		year,
	)
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

	url := "https://name.web3.storage/name/" + peerStr
	log.Debug().Str("url", url).Msg("publishing new name")

	resp, err := s.client.R().SetContext(ctx).
		SetBody(base64.StdEncoding.EncodeToString(encoded)).
		Post(url)
	if err != nil {
		return errors.Wrap(err, "failed to publish new record")
	}

	if resp.StatusCode() != http.StatusAccepted {
		return errors.Errorf(
			"failed to publish new record: %d - %s",
			resp.StatusCode(), resp.Body(),
		)
	}

	if s.lastSequence != nil {
		*s.lastSequence++
	} else {
		s.lastSequence = &sequence
	}

	s.lastCid = &value

	return nil
}

func getLastRecord(
	ctx context.Context,
	log zerolog.Logger,
	client *resty.Client,
	peerCid string,
) (*ipns_pb.IpnsEntry, error) {
	url := "https://name.web3.storage/name/" + peerCid
	log.Debug().Str("url", url).Msg("getting last record")
	resp, err := client.R().SetContext(ctx).Get(url)
	log.Debug().Str("url", url).Dur("timeSpent", resp.Time()).Str(
		"status",
		resp.Status(),
	).Msg("got last record response")

	if err != nil {
		return nil, errors.Wrap(err, "failed to get last cid")
	}

	if resp.StatusCode() != http.StatusOK {
		if strings.Contains(string(resp.Body()), "not found") {
			return nil, nil
		}

		return nil, errors.Errorf(
			"failed to get last cid: %d - %s",
			resp.StatusCode(), resp.Body(),
		)
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
	//nolint:gomnd
	node, err := qp.BuildList(
		basicnode.Prototype.Any, 2, func(la datamodel.ListAssembler) {
			qp.ListEntry(la, qp.Bytes(data))
			if s.lastCid != nil {
				qp.ListEntry(la, qp.Link(cidlink.Link{Cid: *s.lastCid}))
			} else {
				qp.ListEntry(la, qp.Null())
			}
		},
	)
	if err != nil {
		return errors.Wrap(err, "failed to build node")
	}

	buffer := new(bytes.Buffer)

	err = dagcbor.Encode(node, buffer)
	if err != nil {
		return errors.Wrap(err, "failed to encode content")
	}

	url := "https://api.web3.storage/upload"
	s.log.Debug().Str("url", url).Msg("uploading data")
	responseStr, err := s.client.R().SetContext(ctx).SetBody(buffer.Bytes()).Post(url)
	s.log.Debug().Str("url", url).Dur("timeSpent", responseStr.Time()).Str(
		"status",
		responseStr.Status(),
	).Msg("uploaded data response")

	if err != nil {
		return errors.Wrap(err, "failed to upload content")
	}

	if responseStr.StatusCode() != http.StatusOK {
		return errors.Errorf(
			"failed to upload content: %d - %s",
			responseStr.StatusCode(), responseStr.Body(),
		)
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

	return s.publishNewName(ctx, lastCid)
}
