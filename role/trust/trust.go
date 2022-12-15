package trust

import (
	"context"
	"encoding/json"
	"time"

	"validation-bot/store"

	"github.com/google/go-cmp/cmp"
	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/pkg/errors"
	"github.com/rs/zerolog"
	log2 "github.com/rs/zerolog/log"
	"golang.org/x/exp/slices"
)

type (
	OperationType = string
)

const (
	Create OperationType = "create"
	Revoke OperationType = "revoke"
	Reset  OperationType = "reset"
)

type Operation struct {
	Type         OperationType `json:"type"`
	Peer         string        `json:"peer,omitempty"`
	Peers        []string      `json:"peers,omitempty"`
	TrustedPeers []string      `json:"trustedPeers"`
}

func ModifyPeers(
	ctx context.Context,
	publisher store.Publisher,
	subscriber store.Subscriber,
	operationType OperationType,
	trustor peer.ID,
	trustees []peer.ID,
	conflictRetryInterval time.Duration,
) error {
	currentTrustedPeers, err := ListPeers(ctx, subscriber, trustor)
	if err != nil {
		return errors.Wrapf(err, "failed to list peers")
	}

	for {
		trusteesStr := make([]string, len(trustees))
		for i, peerID := range trustees {
			trusteesStr[i] = peerID.String()
		}

		newState := make(map[string]struct{})

		if operationType == Reset {
			for _, peerID := range trustees {
				newState[peerID.String()] = struct{}{}
			}
		} else {
			for _, peerID := range currentTrustedPeers {
				newState[peerID] = struct{}{}
			}

			if operationType == Create {
				for _, peerID := range trustees {
					newState[peerID.String()] = struct{}{}
				}
			} else if operationType == Revoke {
				for _, peerID := range trustees {
					delete(newState, peerID.String())
				}
			}
		}

		trustedPeers := make([]string, 0)
		for peerID := range newState {
			trustedPeers = append(trustedPeers, peerID)
		}

		operation := Operation{
			Type:         operationType,
			Peer:         "",
			Peers:        trusteesStr,
			TrustedPeers: trustedPeers,
		}

		marshal, err := json.Marshal(operation)
		if err != nil {
			return errors.Wrapf(err, "failed to marshal operation")
		}

		err = publisher.Publish(ctx, marshal)
		if err != nil {
			return errors.Wrapf(err, "failed to publish operation")
		}

		currentTrustedPeers, err = ListPeers(ctx, subscriber, trustor)
		if err != nil {
			return errors.Wrapf(err, "failed to list peers")
		}

		if cmp.Equal(currentTrustedPeers, trustedPeers) {
			break
		} else {
			log2.Debug().Msgf("detected conflict. retrying to modify peers")
			time.Sleep(conflictRetryInterval)
		}
	}

	return nil
}

func ListPeers(ctx context.Context, subscriber store.Subscriber, trustor peer.ID) (
	[]string,
	error,
) {
	entry, err := subscriber.Head(ctx, trustor)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to get head for trustor %s", trustor)
	}

	if entry == nil {
		return []string{}, nil
	}

	var operation Operation

	err = json.Unmarshal(entry.Message, &operation)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to unmarshal operation")
	}

	if operation.TrustedPeers == nil {
		return []string{}, nil
	}

	return operation.TrustedPeers, nil
}

type Manager struct {
	Trustors         []peer.ID
	trustees         *map[peer.ID]struct{}
	resultSubscriber store.Subscriber
	log              zerolog.Logger
	retryInterval    time.Duration
	pollInterval     time.Duration
}

func NewManager(
	trustors []peer.ID,
	resultSubscriber store.Subscriber,
	retryInterval time.Duration,
	pollInterval time.Duration,
) *Manager {
	return &Manager{
		Trustors:         trustors,
		trustees:         &map[peer.ID]struct{}{},
		resultSubscriber: resultSubscriber,
		log:              log2.With().Str("role", "trust_manager").Caller().Logger(),
		retryInterval:    retryInterval,
		pollInterval:     pollInterval,
	}
}

func (m *Manager) Trustees() map[peer.ID]struct{} {
	return *m.trustees
}

func (m *Manager) IsTrusted(peerID peer.ID) bool {
	_, ok := (*m.trustees)[peerID]
	if ok {
		return true
	}

	return slices.Contains(m.Trustors, peerID)
}

func (m *Manager) Start(ctx context.Context) {
	go func() {
		for {
			newTrustees := make(map[peer.ID]struct{})

			for _, peerID := range m.Trustors {
				log := m.log.With().Str("peerID", peerID.String()).Logger()

				log.Debug().Msg("start downloading list of trustee peers")

				peers, err := ListPeers(ctx, m.resultSubscriber, peerID)
				if err != nil {
					log.Error().Err(err).Msg("failed to list peers")
					time.Sleep(m.retryInterval)
					continue
				}

				log.Debug().Int("peers", len(peers)).Msg("received list of trustee peers")

				for _, peerIDStr := range peers {
					peerID, err = peer.Decode(peerIDStr)
					if err != nil {
						log.Error().Err(err).Msg("failed to decode peerID")
						continue
					}

					newTrustees[peerID] = struct{}{}
				}
			}

			m.trustees = &newTrustees
			time.Sleep(m.pollInterval)
		}
	}()
}
