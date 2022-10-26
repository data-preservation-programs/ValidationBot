package module

import (
	"context"
	"validation-bot/task"

	"github.com/google/uuid"
	"github.com/jackc/pgtype"
	"gorm.io/gorm"
)

type AuditorModule interface {
	// Validate accepts the task input and returns the validation Result
	Validate(ctx context.Context, input ValidationInput) (*ValidationResult, error)
}

type DispatcherModule interface {
	// Validate returns the validation error for invalid tasks
	Validate(task.Definition) error

	// GetTasks returns task inputs that should be executed according to certain restriction
	// such as priority or number of concurrent task for each target.
	GetTasks([]task.Definition) (map[uuid.UUID]ValidationInput, error)

	// GetTask generates the task input from task definition
	GetTask(task.Definition) (ValidationInput, error)
}

type SimpleDispatcher struct{}
type ValidationInput struct {
	task.Task
	Input pgtype.JSONB `json:"input"`
}

type ValidationResult struct {
	task.Task
	Result pgtype.JSONB `json:"result" gorm:"type:jsonb;default:'{}'"`
}

type ValidationResultModel struct {
	task.Task
	Result      pgtype.JSONB `gorm:"type:jsonb;default:'{}'"`
	Cid         string
	PreviousCid *string
	PeerID      string
	gorm.Model
}

func (ValidationResultModel) TableName() string {
	return "validation_results"
}

func (s SimpleDispatcher) GetTasks(definitions []task.Definition) (map[uuid.UUID]ValidationInput, error) {
	result := make(map[uuid.UUID]ValidationInput)
	for _, definition := range definitions {
		input, _ := s.GetTask(definition)
		result[definition.ID] = input
	}

	return result, nil
}

func (s SimpleDispatcher) GetTask(definition task.Definition) (ValidationInput, error) {
	input := ValidationInput{
		Task: task.Task{
			Type:         definition.Type,
			DefinitionID: definition.ID,
			Target:       definition.Target,
		},
		Input: definition.Definition,
	}
	return input, nil
}

func NewJSONB(input interface{}) (pgtype.JSONB, error) {
	var jsonb pgtype.JSONB
	err := jsonb.Set(input)
	return jsonb, err
}
