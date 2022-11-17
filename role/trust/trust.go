package trust

import (
	"context"
	"encoding/json"
	"sync"
	"time"

	"validation-bot/store"

	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/pkg/errors"
	"github.com/rs/zerolog"
	log2 "github.com/rs/zerolog/log"
)

type (
	Valid         = bool
	OperationType = string
)

const (
	Create OperationType = "create"
	Revoke OperationType = "revoke"
)

type Operation struct {
	Type OperationType `json:"type"`
	Peer string        `json:"peer"`
}

func AddNewPeer(ctx context.Context, publisher store.Publisher, peerID peer.ID) error {
	operation := Operation{
		Type: Create,
		Peer: peerID.String(),
	}

	marshal, err := json.Marshal(operation)
	if err != nil {
		return errors.Wrapf(err, "failed to marshal operation")
	}

	err = publisher.Publish(ctx, marshal)
	if err != nil {
		return errors.Wrapf(err, "failed to publish operation")
	}

	return nil
}

func RevokePeer(ctx context.Context, publisher store.Publisher, peerID peer.ID) error {
	operation := Operation{
		Type: Revoke,
		Peer: peerID.String(),
	}

	marshal, err := json.Marshal(operation)
	if err != nil {
		return errors.Wrapf(err, "failed to marshal operation")
	}

	err = publisher.Publish(ctx, marshal)
	if err != nil {
		return errors.Wrapf(err, "failed to publish operation")
	}

	return nil
}

func ListPeers(ctx context.Context, subscriber store.Subscriber, peerID peer.ID) (
	map[peer.ID]Valid,
	error,
) {
	entries, err := subscriber.Subscribe(ctx, peerID, nil, true)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to subscribe to peer %s", peerID)
	}

	result := make(map[peer.ID]Valid)

	for entry := range entries {
		var operation Operation

		err = json.Unmarshal(entry.Message, &operation)
		if err != nil {
			return nil, errors.Wrapf(err, "failed to unmarshal operation")
		}

		operationPeerID, err := peer.Decode(operation.Peer)
		if err != nil {
			return nil, errors.Wrapf(err, "failed to decode peer")
		}

		switch operation.Type {
		case Create:
			result[operationPeerID] = true
		case Revoke:
			result[operationPeerID] = false
		default:
			return nil, errors.Errorf("unknown operation type %s", operation.Type)
		}
	}

	return result, nil
}

type Manager struct {
	trustors         []peer.ID
	trustees         *map[peer.ID]struct{}
	resultSubscriber store.Subscriber
	log              zerolog.Logger
	started          bool
	startedLock      sync.Mutex
	diffSubscribers  []func(map[peer.ID]struct{})
}

func NewManager(trustors []peer.ID, resultSubscriber store.Subscriber) *Manager {
	return &Manager{
		trustors:         trustors,
		trustees:         &map[peer.ID]struct{}{},
		resultSubscriber: resultSubscriber,
		log:              log2.With().Str("component", "trust-manager").Caller().Logger(),
	}
}

func (m *Manager) Trustees() map[peer.ID]struct{} {
	return *m.trustees
}

func (m *Manager) IsTrusted(peerID peer.ID) bool {
	_, ok := (*m.trustees)[peerID]
	return ok
}

func (m *Manager) SubscribeToDiff(f func(map[peer.ID]struct{})) {
	m.diffSubscribers = append(m.diffSubscribers, f)
}

func (m *Manager) Start(ctx context.Context) {
	m.startedLock.Lock()
	if m.started {
		m.startedLock.Unlock()
		return
	}
	m.startedLock.Unlock()

	go func() {
		for {
			newTrustees := make(map[peer.ID]struct{})

			for _, peerID := range m.trustors {
				log := m.log.With().Str("peerID", peerID.String()).Logger()

				log.Debug().Msg("start downloading list of trustee peers")

				peers, err := ListPeers(ctx, m.resultSubscriber, peerID)
				if err != nil {
					log.Error().Err(err).Msg("failed to list peers")
					time.Sleep(time.Minute)
					continue
				}

				log.Debug().Int("peers", len(peers)).Msg("received list of trustee peers")

				for peerID, valid := range peers {
					if valid {
						newTrustees[peerID] = struct{}{}
					}
				}
			}

			diff := make(map[peer.ID]struct{})
			oldTrustees := *m.trustees
			for peerID := range newTrustees {
				if _, ok := oldTrustees[peerID]; !ok {
					diff[peerID] = struct{}{}
				}
			}

			m.trustees = &newTrustees
			m.log.Debug().Int("peers", len(diff)).Msg("new auditor peers")
			for _, subscriber := range m.diffSubscribers {
				subscriber(diff)
			}

			time.Sleep(time.Minute)
		}
	}()
}
