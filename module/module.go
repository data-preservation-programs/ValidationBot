package module

import (
	"context"
	"time"

	"validation-bot/task"

	"github.com/google/uuid"
	"github.com/jackc/pgtype"
	"github.com/pkg/errors"
	"gorm.io/gorm"
)

type AuditorModule interface {
	// Validate accepts the task input and returns the validation Result
	Validate(ctx context.Context, input ValidationInput) (*ValidationResult, error)

	Type() task.Type
}

type DispatcherModule interface {
	// Validate returns the validation error for invalid tasks
	Validate(task.Definition) error

	// GetTask generates the task input from task definition
	GetTask(task.Definition) (*ValidationInput, error)

	Type() task.Type
}

type (
	SimpleDispatcher struct{}
	ValidationInput  struct {
		task.Task
		Input pgtype.JSONB `json:"input" gorm:"type:jsonb;default:'{}'"`
	}
	NoopValidator     struct{}
	DefaultDispatcher struct {
		SimpleDispatcher
		NoopValidator
	}
)

type ValidationResult struct {
	ValidationInput
	Result pgtype.JSONB `json:"result" gorm:"type:jsonb;default:'{}'"`
}

type ValidationResultModel struct {
	CreatedAt time.Time `gorm:"index:idx_createdAt_type_target"`
	ValidationInput
	Result      pgtype.JSONB `gorm:"type:jsonb;default:'{}'"`
	Cid         string
	PreviousCid *string
	PeerID      string
	ID          uint `gorm:"primarykey"`
	UpdatedAt   time.Time
	DeletedAt   gorm.DeletedAt `gorm:"index"`
}

func (ValidationResultModel) TableName() string {
	return "validation_results"
}

func (NoopValidator) Validate(task.Definition) error {
	return nil
}

func (s SimpleDispatcher) GetTask(definition task.Definition) (*ValidationInput, error) {
	input := ValidationInput{
		Task: task.Task{
			Type:         definition.Type,
			DefinitionID: definition.ID,
			Target:       definition.Target,
			Tag:          definition.Tag,
			TaskID:       uuid.New(),
		},
		Input: definition.Definition,
	}
	return &input, nil
}

func NewJSONB(input interface{}) (pgtype.JSONB, error) {
	var jsonb pgtype.JSONB
	err := jsonb.Set(input)
	return jsonb, errors.Wrap(err, "failed to set jsonb")
}
