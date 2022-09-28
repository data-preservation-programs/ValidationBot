package echo

import (
	"context"
	"encoding/json"

	"validation-bot/task"

	"github.com/pkg/errors"
	"github.com/rs/zerolog/log"
)

type Task struct {
	task.Task
	Input string `json:"input"`
}

type Result struct {
	task.Task
	Echo string `json:"echo"`
}

type Validator struct{}

func (v Validator) Validate(ctx context.Context, input []byte) ([]byte, error) {
	log.Debug().Msg("echo validator called")

	var task Task

	err := json.Unmarshal(input, &task)
	if err != nil {
		return nil, errors.Wrap(err, "failed to unmarshal task")
	}

	result := Result{
		Task: task.Task,
		Echo: task.Input,
	}

	output, err := json.Marshal(result)
	if err != nil {
		return nil, errors.Wrap(err, "failed to marshal result")
	}

	log.Debug().Msg("echo validator finished")

	return output, nil
}
