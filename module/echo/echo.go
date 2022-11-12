package echo

import (
	"context"

	"validation-bot/module"

	"github.com/rs/zerolog"
	log2 "github.com/rs/zerolog/log"
)

type Dispatcher struct {
	module.SimpleDispatcher
	module.NoopValidator
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
		ValidationInput: input,
		Result:          input.Input,
	}
	return &output, nil
}
