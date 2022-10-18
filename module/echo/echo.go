package echo

import (
	"context"
	"validation-bot/module"
	"validation-bot/task"
)

type Echo struct {
	module.SimpleDispatcher
}

func (e Echo) Validate(ctx context.Context, input module.ValidationInput) (*module.ValidationResult, error) {
	output := module.ValidationResult{
		Task:   input.Task,
		Result: input.Input,
	}
	return &output, nil
}

func (Echo) TaskType() task.Type {
	return task.Echo
}
