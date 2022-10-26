package echo

import (
	"context"
	"validation-bot/module"
	"validation-bot/task"

	"github.com/rs/zerolog"
	log2 "github.com/rs/zerolog/log"
)

type Dispatcher struct {
	module.SimpleDispatcher
}

func (d Dispatcher) Validate(definition task.Definition) error {
	return nil
}

type Auditor struct {
	log zerolog.Logger
}

func NewEchoAuditor() Auditor {
	return Auditor{
		log: log2.With().Str("role", "echo_auditor").Caller().Logger(),
	}
}

func (e Auditor) Validate(ctx context.Context, input module.ValidationInput) (*module.ValidationResult, error) {
	e.log.Info().Bytes("definition", input.Input.Bytes).Msgf("performing validation")
	output := module.ValidationResult{
		Task:   input.Task,
		Result: input.Input,
	}
	return &output, nil
}
