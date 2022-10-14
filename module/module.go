package module

import (
	"context"

	"validation-bot/task"
)

type Module interface {
	AuditorModule
	AuditorModule
	ObserverModule
}

type AuditorModule interface {
	// TaskType is a field that can be used to identify the test type
	TaskType() task.Type

	// Validate accepts the task input and returns the validation result
	Validate(ctx context.Context, input []byte) ([]byte, error)
}

type ObserverModule interface {
	// TaskType is a field that can be used to identify the test type
	TaskType() task.Type

	// ResultType is the type of the result that can be used to create database tables
	ResultType() interface{}
}

type DispatcherModule interface {
	// TaskType is a field that can be used to identify the test type
	TaskType() task.Type

	// GetTasks returns task inputs that should be executed according to certain restriction
	// such as priority or number of concurrent task for each target.
	GetTasks([]task.Definition) (map[task.Definition][]byte, error)

	// GetTask generates the task input from task definition
	GetTask(task.Definition) ([]byte, error)
}
