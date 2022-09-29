package auditor

import (
	"context"
	"encoding/json"

	"validation-bot/module"

	"validation-bot/task"

	"validation-bot/store"

	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/pkg/errors"
	"github.com/rs/zerolog/log"
	"golang.org/x/exp/slices"
)

type Auditor struct {
	modules         map[task.Type]module.Module
	trustedPeers    []peer.ID
	resultPublisher store.ResultPublisher
	taskSubscriber  task.Subscriber
}

type Config struct {
	TrustedPeers    []peer.ID
	ResultPublisher store.ResultPublisher
	TaskSubscriber  task.Subscriber
	Modules         []module.Module
}

func NewAuditor(config Config) (*Auditor, error) {
	modules := make(map[task.Type]module.Module)
	for _, mod := range config.Modules {
		modules[mod.TaskType()] = mod
	}

	auditor := Auditor{
		modules:         modules,
		trustedPeers:    config.TrustedPeers,
		resultPublisher: config.ResultPublisher,
		taskSubscriber:  config.TaskSubscriber,
	}

	return &auditor, nil
}

func (a Auditor) Start(ctx context.Context) <-chan error {
	log.Info().Msg("start listening to subscription")
	errChannel := make(chan error)

	go func() {
		for {
			from, task, err := a.taskSubscriber.Next(ctx)
			if err != nil {
				errChannel <- errors.Wrap(err, "failed to receive next message")
			}

			if len(a.trustedPeers) > 0 && !slices.Contains(a.trustedPeers, *from) {
				log.Warn().Msg("received message from untrusted peer")
				continue
			}

			log.Info().Str("from", from.String()).Interface("task", task).Msg("received message")

			err = a.handleValidationTask(ctx, task)
			if err != nil {
				errChannel <- errors.Wrap(err, "failed to handle validation task")
			}
		}
	}()

	return errChannel
}

func (a Auditor) handleValidationTask(ctx context.Context, taskMessage []byte) error {
	msg := new(task.Task)

	err := json.Unmarshal(taskMessage, msg)
	if err != nil {
		return errors.Wrap(err, "unable to unmarshal the message to generic task")
	}

	mod, ok := a.modules[msg.Type]
	if !ok {
		return errors.Errorf("module task type is unsupported: %s", msg.Type)
	}

	result, err := mod.Validate(ctx, taskMessage)
	if err != nil {
		return errors.Wrap(err, "encountered error performing module")
	}

	return a.publishResult(ctx, result)
}

func (a Auditor) publishResult(ctx context.Context, result []byte) error {
	log.Info().Str("result", string(result)).Msg("publishing result")

	err := a.resultPublisher.Publish(ctx, result)
	if err != nil {
		return errors.Wrap(err, "failed to publish result")
	}

	return nil
}
