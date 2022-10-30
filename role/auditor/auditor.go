package auditor

import (
	"context"
	"encoding/json"

	"validation-bot/module"

	"validation-bot/task"

	"validation-bot/store"

	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/pkg/errors"
	"github.com/rs/zerolog"
	log2 "github.com/rs/zerolog/log"
	"golang.org/x/exp/slices"
)

type Auditor struct {
	modules         map[task.Type]module.AuditorModule
	trustedPeers    []peer.ID
	resultPublisher store.ResultPublisher
	taskSubscriber  task.Subscriber
	log             zerolog.Logger
}

type Config struct {
	TrustedPeers    []peer.ID
	ResultPublisher store.ResultPublisher
	TaskSubscriber  task.Subscriber
	Modules         map[task.Type]module.AuditorModule
}

func NewAuditor(config Config) (*Auditor, error) {
	log := log2.With().Str("role", "auditor").Caller().Logger()

	auditor := Auditor{
		modules:         config.Modules,
		trustedPeers:    config.TrustedPeers,
		resultPublisher: config.ResultPublisher,
		taskSubscriber:  config.TaskSubscriber,
		log:             log,
	}

	return &auditor, nil
}

func (a Auditor) Start(ctx context.Context) <-chan error {
	log := a.log
	errChannel := make(chan error)

	log.Info().Msg("start listening to subscription")

	go func() {
		for {
			log.Debug().Msg("waiting for task")

			from, task, err := a.taskSubscriber.Next(ctx)
			if err != nil {
				errChannel <- errors.Wrap(err, "failed to receive next message")
			}

			if len(a.trustedPeers) > 0 && !slices.Contains(a.trustedPeers, *from) {
				log.Debug().Str("from", from.String()).Msg("received message from untrusted peer")
				continue
			}

			log.Info().Str("from", from.String()).Bytes("task", task).Msg("received a new task")

			input := new(module.ValidationInput)

			err = json.Unmarshal(task, input)
			if err != nil {
				errChannel <- errors.Wrap(err, "unable to unmarshal the message")
			}

			mod, ok := a.modules[input.Type]
			if !ok {
				errChannel <- errors.Errorf("module task type is unsupported: %s", input.Type)
			}

			go func() {
				log.Debug().Bytes("task", task).Msg("performing validation")

				result, err := mod.Validate(ctx, *input)
				if err != nil {
					errChannel <- errors.Wrap(err, "encountered error performing module")
				}

				if result == nil {
					log.Info().Msg("validation result is nil, skipping publishing")
					return
				}

				log.Debug().Int("resultSize", len(result.Result.Bytes)).Msg("validation completed")

				resultBytes, err := json.Marshal(result)
				if err != nil {
					errChannel <- errors.Wrap(err, "unable to marshal result")
				}

				err = a.resultPublisher.Publish(ctx, resultBytes)
				if err != nil {
					errChannel <- errors.Wrap(err, "failed to publish result")
				}
			}()
		}
	}()

	return errChannel
}
