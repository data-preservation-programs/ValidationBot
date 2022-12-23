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

	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/pkg/errors"
	"github.com/rs/zerolog"
	log2 "github.com/rs/zerolog/log"
)

type Auditor struct {
	peerID                  peer.ID
	modules                 map[task.Type]module.AuditorModule
	trustManager            *trust.Manager
	resultPublisher         store.Publisher
	taskPublisherSubscriber task.PublisherSubscriber
	log                     zerolog.Logger
	bidding                 map[task.DefinitionID]map[peer.ID]uint64
	biddingLock             sync.RWMutex
	biddingWait             time.Duration
	rpcClient               IRPCClient
	RPCTimeout              time.Duration
}

type Config struct {
	PeerID                  peer.ID
	TrustManager            *trust.Manager
	ResultPublisher         store.Publisher
	TaskPublisherSubscriber task.PublisherSubscriber
	Modules                 map[task.Type]module.AuditorModule
	BiddingWait             time.Duration
	RPCClient               IRPCClient
	RPCTimeout              time.Duration
}

type Bidding struct {
	Type   string      `json:"type"`
	Value  uint64      `json:"value"`
	TaskID task.TaskID `json:"taskId"`
}

func NewAuditor(config Config) (*Auditor, error) {
	log := log2.With().Str("role", "auditor").Caller().Logger()

	auditor := Auditor{
		peerID:                  config.PeerID,
		modules:                 config.Modules,
		trustManager:            config.TrustManager,
		resultPublisher:         config.ResultPublisher,
		taskPublisherSubscriber: config.TaskPublisherSubscriber,
		log:                     log,
		bidding:                 make(map[task.DefinitionID]map[peer.ID]uint64),
		biddingLock:             sync.RWMutex{},
		biddingWait:             config.BiddingWait,
		rpcClient:               config.RPCClient,
		RPCTimeout:              config.RPCTimeout,
	}

	return &auditor, nil
}

func (a *Auditor) Start(ctx context.Context) {
	log := a.log
	log.Info().Msg("start listening to subscription")

	go func() {
		for {
			log.Debug().Msg("waiting for task")

			from, task, err := a.taskPublisherSubscriber.Next(ctx)
			if err != nil {
				log.Error().Err(err).Msg("failed to get next task")
				time.Sleep(time.Minute)
				continue
			}

			log.Info().Str("from", from.String()).Bytes("message", task).Msg("received a new message")

			if !a.trustManager.IsTrusted(*from) {
				a.log.Debug().Str("from", from.String()).Msg("received message from untrusted peer")
				return
			}

			// If the message is a bidding message
			bidding := new(Bidding)

			err = json.Unmarshal(task, bidding)
			if err != nil {
				log.Error().Err(err).Msg("failed to unmarshal bidding")
				continue
			}

			if bidding.Type == "bidding" {
				a.handleBiddingMessage(bidding, from)
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

			shouldValidate, err := mod.ShouldValidate(ctx, *input)
			if err != nil {
				log.Error().Err(err).Msg("failed to check if we should validate")
				continue
			}

			if !shouldValidate {
				log.Debug().Msg("validation not performed because shouldValidate returned false")
				continue
			}

			// Make bidding
			err = a.makeBidding(ctx, input)
			if err != nil {
				log.Error().Err(err).Msg("failed to make bidding")
				continue
			}

			go func() {
				won := a.resolveBidding(task, bidding)
				if !won {
					return
				}

				log.Debug().Bytes("task", task).Msg("performing validation")

				// mod.Validate
				// test run graphSync.Validate in a loop -- watch memory run away
				// TODO which timeout?
				ctx, cancel := context.WithTimeout(ctx, a.RPCTimeout)
				defer cancel()

				result, err := a.rpcClient.Call(ctx, *input)

				if errors.Is(err, context.DeadlineExceeded) {
					log.Error().Bytes("task", task).Err(err).Msg("validation timed out")
				}

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

func (a *Auditor) resolveBidding(task []byte, bidding *Bidding) bool {
	log := a.log

	log.Debug().Bytes("task", task).Msg("waiting for all bids before performing validation")
	time.Sleep(a.biddingWait)

	defer func() {
		a.biddingLock.Lock()
		delete(a.bidding, bidding.TaskID)
		a.biddingLock.Unlock()
	}()

	maxBid := uint64(0)

	a.biddingLock.RLock()

	for _, bid := range a.bidding[bidding.TaskID] {
		if bid > maxBid {
			maxBid = bid
		}
	}

	won := true

	if maxBid != a.bidding[bidding.TaskID][a.peerID] {
		log.Debug().Bytes(
			"task",
			task,
		).Msg("validation not performed because we did not win the bidding")

		won = false
	}

	a.biddingLock.RUnlock()
	return won
}

func (a *Auditor) handleBiddingMessage(bidding *Bidding, from *peer.ID) {
	a.biddingLock.RLock()
	if _, ok := a.bidding[bidding.TaskID]; !ok {
		a.log.Debug().Str(
			"from",
			from.String(),
		).Msg("received bidding message but the bidding has already been resolved")
		a.biddingLock.RUnlock()
		return
	}

	a.biddingLock.RUnlock()
	a.biddingLock.Lock()
	a.bidding[bidding.TaskID][*from] = bidding.Value
	a.biddingLock.Unlock()
	a.log.Debug().Str("taskId", bidding.TaskID.String()).Str("from", from.String()).Uint64(
		"bid",
		bidding.Value,
	).Msg("received bidding")
}

func (a *Auditor) makeBidding(ctx context.Context, input *module.ValidationInput) error {
	randomNumber, err := rand.Int(rand.Reader, big.NewInt(math.MaxUint32))
	a.log.Debug().Str("task_id", input.TaskID.String()).Uint64("bid", randomNumber.Uint64()).Msg("making bidding")

	if err != nil {
		return errors.Wrap(err, "failed to generate random number")
	}

	bidding := &Bidding{
		Type:   "bidding",
		Value:  randomNumber.Uint64(),
		TaskID: input.TaskID,
	}

	biddingStr, err := json.Marshal(bidding)
	if err != nil {
		return errors.Wrap(err, "failed to marshal bidding")
	}

	a.biddingLock.Lock()
	a.bidding[bidding.TaskID] = make(map[peer.ID]uint64)
	a.bidding[bidding.TaskID][a.peerID] = bidding.Value
	a.biddingLock.Unlock()

	err = a.taskPublisherSubscriber.Publish(ctx, biddingStr)
	if err != nil {
		return errors.Wrap(err, "failed to publish bidding")
	}

	return nil
}
