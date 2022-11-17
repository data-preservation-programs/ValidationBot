package auditor

import (
	"context"
	"crypto/rand"
	"encoding/json"
	"math"
	"math/big"
	"sync"
	"time"
	"validation-bot/role/trust"

	"validation-bot/module"

	"validation-bot/task"

	"validation-bot/store"

	"github.com/google/uuid"
	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/rs/zerolog"
	log2 "github.com/rs/zerolog/log"
	"golang.org/x/exp/slices"
)

type Auditor struct {
	peerID                 peer.ID
	modules                map[task.Type]module.AuditorModule
	trustedDispatcherPeers []peer.ID
	trustManager           trust.Manager
	resultPublisher        store.Publisher
	taskSubscriber         task.Subscriber
	taskPublisher          task.Publisher
	log                    zerolog.Logger
	bidding                map[uuid.UUID]map[peer.ID]uint64
	biddingLock            sync.RWMutex
	biddingWait            time.Duration
}

type Config struct {
	PeerID                 peer.ID
	TrustedDispatcherPeers []peer.ID
	TrustManager           trust.Manager
	ResultPublisher        store.Publisher
	TaskSubscriber         task.Subscriber
	TaskPublisher          task.Publisher
	Modules                map[task.Type]module.AuditorModule
	BiddingWait            time.Duration
}

type Bidding struct {
	Type   string    `json:"type"`
	Value  uint64    `json:"value"`
	TaskID uuid.UUID `json:"taskId"`
}

func NewAuditor(config Config) (*Auditor, error) {
	log := log2.With().Str("role", "auditor").Caller().Logger()

	auditor := Auditor{
		peerID:                 config.PeerID,
		modules:                config.Modules,
		trustedDispatcherPeers: config.TrustedDispatcherPeers,
		resultPublisher:        config.ResultPublisher,
		taskSubscriber:         config.TaskSubscriber,
		taskPublisher:          config.TaskPublisher,
		trustManager:           config.TrustManager,
		log:                    log,
		bidding:                make(map[uuid.UUID]map[peer.ID]uint64),
		biddingWait:            config.BiddingWait,
	}

	return &auditor, nil
}

func (a Auditor) Start(ctx context.Context) {
	a.trustManager.Start(ctx)
	log := a.log
	log.Info().Msg("start listening to subscription")

	go func() {
		for {
			log.Debug().Msg("waiting for task")

			from, task, err := a.taskSubscriber.Next(ctx)
			if err != nil {
				log.Error().Err(err).Msg("failed to get next task")
				time.Sleep(time.Minute)
				continue
			}

			log.Info().Str("from", from.String()).Bytes("message", task).Msg("received a new message")
			bidding := new(Bidding)
			err = json.Unmarshal(task, bidding)
			if err != nil {
				log.Error().Err(err).Msg("failed to unmarshal bidding")
				continue
			}

			if bidding.Type == "bidding" {
				if !a.trustManager.IsTrusted(*from) {
					log.Debug().Str("from", from.String()).Msg("received bidding message from untrusted peer")
					continue
				}

				{
					a.biddingLock.Lock()
					if _, ok := a.bidding[bidding.TaskID]; !ok {
						a.bidding[bidding.TaskID] = make(map[peer.ID]uint64)
					}

					a.bidding[bidding.TaskID][*from] = bidding.Value
					a.biddingLock.Unlock()
				}

				continue
			}

			if !slices.Contains(a.trustedDispatcherPeers, *from) {
				log.Debug().Str("from", from.String()).Msg("received task from untrusted peer")
				continue
			}

			input := new(module.ValidationInput)

			err = json.Unmarshal(task, input)
			if err != nil {
				log.Error().Err(err).Msg("failed to unmarshal task")
				continue
			}

			mod, ok := a.modules[input.Type]
			if !ok {
				log.Error().Err(err).Msg("module not found")
				continue
			}

			// Publish bidding
			randomNumber, err := rand.Int(rand.Reader, big.NewInt(math.MaxUint64))
			if err != nil {
				log.Error().Err(err).Msg("failed to generate random number")
				continue
			}

			bidding = &Bidding{
				Type:   "bidding",
				Value:  randomNumber.Uint64(),
				TaskID: input.TaskID,
			}

			biddingStr, err := json.Marshal(bidding)
			if err != nil {
				log.Error().Err(err).Msg("failed to marshal bidding")
				continue
			}

			{
				a.biddingLock.Lock()
				a.bidding[bidding.TaskID] = make(map[peer.ID]uint64)
				a.bidding[bidding.TaskID][a.peerID] = bidding.Value
				a.biddingLock.Unlock()
			}

			err = a.taskPublisher.Publish(ctx, biddingStr)
			if err != nil {
				log.Error().Err(err).Msg("failed to publish bidding")
				continue
			}

			go func() {
				log.Debug().Bytes("task", task).Msg("waiting for all bids before performing validation")
				time.Sleep(a.biddingWait)
				defer func() {
					a.biddingLock.Lock()
					delete(a.bidding, bidding.TaskID)
					a.biddingLock.Unlock()
				}()

				{
					maxBid := uint64(0)
					a.biddingLock.RLock()
					for _, bid := range a.bidding[bidding.TaskID] {
						if bid > maxBid {
							maxBid = bid
						}
					}

					if maxBid != a.bidding[bidding.TaskID][a.peerID] {
						log.Debug().Bytes(
							"task",
							task,
						).Msg("validation not performed because we did not win the bidding")
						a.biddingLock.RUnlock()
						return
					}

					a.biddingLock.RUnlock()
				}

				log.Debug().Bytes("task", task).Msg("performing validation")

				result, err := mod.Validate(ctx, *input)
				if err != nil {
					log.Error().Bytes("task", task).Err(err).Msg("failed to validate")
					return
				}

				if result == nil {
					log.Info().Bytes("task", task).Msg("validation result is nil, skipping publishing")
					return
				}

				log.Debug().Bytes("task", task).Int("resultSize", len(result.Result.Bytes)).Msg("validation completed")

				resultBytes, err := json.Marshal(result)
				if err != nil {
					log.Error().Bytes("task", task).Err(err).Msg("failed to marshal result")
					return
				}

				err = a.resultPublisher.Publish(ctx, resultBytes)
				if err != nil {
					log.Error().Bytes("task", task).Err(err).Msg("failed to publish result")
					return
				}
			}()
		}
	}()
}
