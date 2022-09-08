package model

type ErrorCode string

const (
	NoError                  ErrorCode = ""
	RetrievalQueryFailed     ErrorCode = "retrieval_query_failed"
	QueryResponseUnavailable ErrorCode = "query_response_unavailable"
	QueryResponseError       ErrorCode = "query_response_error"
	InternalError            ErrorCode = "internal_error"
	RetrievalFailed          ErrorCode = "retrieval_failed"
)
