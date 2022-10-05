package echo

import (
	"context"
	"encoding/json"

	"validation-bot/module"

	"validation-bot/task"

	"github.com/pkg/errors"
	"github.com/rs/zerolog/log"
)

type Echo struct{}

func (e Echo) GetTasks(definitions []task.Definition) (map[task.Definition]module.Marshallable, error) {
	result := make(map[task.Definition]module.Marshallable)
	for _, definition := range definitions {
		input, _ := e.GetTask(definition)
		result[definition] = input
	}

	return result, nil
}

func (e Echo) GetTask(definition task.Definition) (module.Marshallable, error) {
	return Input{
		Task: task.Task{
			Type:         task.Echo,
			DefinitionID: definition.ID,
			Target:       definition.Target,
		},
		Input: definition.Definition,
	}, nil
}

func (e Echo) TaskType() task.Type {
	return task.Echo
}

func (e Echo) ResultType() interface{} {
	return &EchoResult{}
}

func (e Echo) Validate(ctx context.Context, input []byte) ([]byte, error) {
	log.Debug().Msg("echo validator called")

	var in Input

	err := json.Unmarshal(input, &in)
	if err != nil {
		return nil, errors.Wrap(err, "failed to unmarshal input")
	}

	output := ResultContent{
		Task:   in.Task,
		Output: in.Input,
	}

	out, err := json.Marshal(output)
	if err != nil {
		return nil, errors.Wrap(err, "failed to marshal output")
	}

	log.Debug().Msg("echo validator finished")

	return out, nil
}
