package echo

import (
	"context"
	"encoding/json"

	"validation-bot/task"

	"github.com/pkg/errors"
)

type Echo struct{}

func (e Echo) GetTasks(definitions []task.Definition) (map[task.Definition][]byte, error) {
	result := make(map[task.Definition][]byte)
	for _, definition := range definitions {
		input, _ := e.GetTask(definition)
		result[definition] = input
	}

	return result, nil
}

func (e Echo) GetTask(definition task.Definition) ([]byte, error) {
	input := Input{
		Task: task.Task{
			Type:         task.Echo,
			DefinitionID: definition.ID,
			Target:       definition.Target,
		},
		Input: definition.Definition,
	}
	return json.Marshal(input)
}

func (e Echo) TaskType() task.Type {
	return task.Echo
}

func (e Echo) ResultType() interface{} {
	return &ResultModel{}
}

func (e Echo) Validate(ctx context.Context, input []byte) ([]byte, error) {
	var in Input

	err := json.Unmarshal(input, &in)
	if err != nil {
		return nil, errors.Wrap(err, "failed to unmarshal input")
	}

	output := Result{
		Task:   in.Task,
		Output: in.Input,
	}

	out, err := json.Marshal(output)
	if err != nil {
		return nil, errors.Wrap(err, "failed to marshal output")
	}

	return out, nil
}
