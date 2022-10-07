package task

import (
	"time"

	"github.com/google/uuid"
)

type Type = string

const (
	Echo Type = "echo"
)

type Task struct {
	Type         Type      `json:"type"`
	DefinitionID uuid.UUID `json:"definitionId"`
	Target       string    `json:"target"`
}

type Definition struct {
	ID              uuid.UUID `json:"id" gorm:"primarykey;type:uuid;default:gen_random_uuid()"`
	Target          string    `json:"target" gorm:"index:idx_type_target"`
	Type            Type      `json:"type" gorm:"index:idx_type_target"`
	IntervalSeconds uint32    `json:"intervalSeconds"`
	Definition      string    `json:"definition"`
	DispatchedTimes uint32
	CreatedAt       time.Time
	UpdatedAt       time.Time
}
