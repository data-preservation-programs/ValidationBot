package task

import (
	"time"

	"github.com/google/uuid"
)

type Type = string

const (
	EchoType Type = "echo"
)

type Task struct {
	Type         Type      `json:"type"`
	Target       string    `json:"target"`
	DefinitionID uuid.UUID `json:"definition_id"`
}

type Definition struct {
	ID              uuid.UUID `json:"id" gorm:"primarykey;type:uuid;default:uuid_generate_v4()"`
	Target          string    `json:"target" gorm:"index:idx_type_target"`
	Type            Type      `json:"type" gorm:"index:idx_type_target"`
	IntervalSeconds uint32    `json:"interval_seconds"`
	Definition      string    `json:"definition"`
	DispatchedTimes uint32
	CreatedAt       time.Time
	UpdatedAt       time.Time
}
