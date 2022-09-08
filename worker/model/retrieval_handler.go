package model

import (
	"context"
)

type RetrievalHandler interface {
	HandleRetrieval(ctx context.Context, message ValidationInput) (ValidationResult, error)
}
