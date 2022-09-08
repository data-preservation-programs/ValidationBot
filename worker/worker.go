package main

import (
	"context"
	"fmt"
	"github.com/aws/aws-lambda-go/lambda"
	"validation-bot/worker/model"
	"validation-bot/worker/retrieval"
)

func HandleRequest(ctx context.Context, message model.ValidationInput) (*model.ValidationResult, error) {
	switch message.Type {
	case "retrieval_graphsync":
		result, err := retrieval.GraphSyncRetrievalHandler{}.HandleRetrieval(ctx, message)
		if err != nil {
			return &model.ValidationResult{
				Success:      false,
				ErrorCode:    model.InternalError,
				ErrorMessage: err.Error(),
			}, nil
		}
		return result, nil
	}
	return nil, fmt.Errorf("unknown validation type: %s", message.Type)
}

func main() {
	lambda.Start(HandleRequest)
}
