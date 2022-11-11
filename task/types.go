package task

import (
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgtype"
)

type Type = string

const (
	Echo          Type = "echo"
	QueryAsk      Type = "query_ask"
	Retrieval     Type = "retrieval"
	Traceroute    Type = "traceroute"
	IndexProvider Type = "index_provider"
)

type Task struct {
	Type         Type      `json:"type"`
	DefinitionID uuid.UUID `json:"definitionId"`
	Target       string    `json:"target"`
	Tag          string    `json:"tag,omitempty"`
}

type Definition struct {
	ID              uuid.UUID    `json:"id" gorm:"primarykey;type:uuid;default:gen_random_uuid()"`
	Target          string       `json:"target" gorm:"index:idx_type_target"`
	Type            Type         `json:"type" gorm:"index:idx_type_target"`
	IntervalSeconds uint32       `json:"intervalSeconds"`
	Definition      pgtype.JSONB `json:"definition" gorm:"type:jsonb;default:'{}'"`
	Tag             string       `json:"tag"`
	DispatchedTimes uint32
	CreatedAt       time.Time
	UpdatedAt       time.Time
}
