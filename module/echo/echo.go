package echo

import (
	"context"
	"validation-bot/module"
	"validation-bot/task"
)

type Dispatcher struct {
	module.SimpleDispatcher
}

func (d Dispatcher) Validate(definition task.Definition) error {
	return nil
}

type Auditor struct {
}

func (e Auditor) Validate(ctx context.Context, input module.ValidationInput) (*module.ValidationResult, error) {
	output := module.ValidationResult{
		Task:   input.Task,
		Result: input.Input,
	}
	return &output, nil
}
