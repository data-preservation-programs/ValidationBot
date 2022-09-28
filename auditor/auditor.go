package auditor

import (
	"context"
	"encoding/json"
	"reflect"

	"validation-bot/task"

	"validation-bot/store"

	"validation-bot/auditor/echo"

	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/pkg/errors"
	"github.com/rs/zerolog/log"
	"golang.org/x/exp/slices"
)

type Validator interface {
	Validate(ctx context.Context, input []byte) (output []byte, err error)
}

type Auditor struct {
	validators      map[task.Type]Validator
	trustedPeers    []peer.ID
	resultPublisher store.ResultPublisher
	taskSubscriber  task.Subscriber
}

type Config struct {
	trustedPeers    []peer.ID
	resultPublisher store.ResultPublisher
	taskSubscriber  task.Subscriber
}

func NewAuditor(config Config) (*Auditor, error) {
	validators := make(map[task.Type]Validator)
	validators[task.EchoType] = echo.Validator{}

	auditor := Auditor{
		validators:      validators,
		trustedPeers:    config.trustedPeers,
		resultPublisher: config.resultPublisher,
		taskSubscriber:  config.taskSubscriber,
	}

	return &auditor, nil
}

func (a Auditor) Start(ctx context.Context) error {
	log.Info().Msg("start listening to subscription")

	for {
		from, task, err := a.taskSubscriber.Next(ctx)
		if err != nil {
			return errors.Wrap(err, "failed to receive next message")
		}

		if len(a.trustedPeers) > 0 && !slices.Contains(a.trustedPeers, *from) {
			log.Warn().Msg("received message from untrusted peer")
			continue
		}

		log.Info().Str("from", from.String()).Interface("task", task).Msg("received message")

		err = a.handleValidationTask(ctx, task)
		if err != nil {
			log.Error().Err(err).Msg("failed to handle validation task")
		}
	}
}

func (a Auditor) handleValidationTask(ctx context.Context, taskMessage []byte) error {
	msg := make(map[string]interface{})

	err := json.Unmarshal(taskMessage, &msg)
	if err != nil {
		return errors.Wrap(err, "unable to unmarshal the message to generic task")
	}

	var taskTypeString string

	taskType, ok := msg["type"]
	if !ok {
		return errors.New("type field is not found")
	}

	switch taskType := taskType.(type) {
	case string:
		taskTypeString = taskType
	default:
		return errors.Errorf("type has an unknown type: %s", reflect.TypeOf(taskType))
	}

	validator, ok := a.validators[taskTypeString]
	if !ok {
		return errors.Errorf("validation task type is unsupported: %s", taskTypeString)
	}

	result, err := validator.Validate(ctx, taskMessage)
	if err != nil {
		return errors.Wrap(err, "encountered error performing validation")
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
