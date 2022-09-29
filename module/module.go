package module

import (
	"context"

	"validation-bot/task"
)

type Marshallable interface {
	Marshal() ([]byte, error)
}

type Module interface {
	TaskType() task.Type
	ResultType() interface{}
	GetTasks([]task.Definition) (map[task.Definition]Marshallable, error)
	GetTask(task.Definition) (Marshallable, error)
	Validate(ctx context.Context, input []byte) ([]byte, error)
}
