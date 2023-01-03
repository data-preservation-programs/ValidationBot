package echo

import (
	"context"

	"validation-bot/task"

	"validation-bot/module"

	"github.com/rs/zerolog"
	log2 "github.com/rs/zerolog/log"
)

type Dispatcher struct {
	module.SimpleDispatcher
	module.NoopValidator
}

func (Dispatcher) Type() task.Type {
	return task.Echo
}

type Auditor struct {
	log zerolog.Logger
}

func (Auditor) Type() task.Type {
	return task.Echo
}

func NewEchoAuditor() Auditor {
	return Auditor{
		log: log2.With().Str("role", "echo_auditor").Caller().Logger(),
	}
}

func (e Auditor) Validate(ctx context.Context, input module.ValidationInput) (*module.ValidationResult, error) {
	e.log.Info().Bytes("definition", input.Input.Bytes).Msgf("performing validation")

	// TODO ipld here?
	output := module.ValidationResult{
		ValidationInput: input,
		Result:          input.Input,
	}
	return &output, nil
}

func (e Auditor) ShouldValidate(ctx context.Context, input module.ValidationInput) (bool, error) {
	return true, nil
}
