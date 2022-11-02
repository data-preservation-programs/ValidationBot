package retrieval

import "time"

type Protocol string

const (
	GraphSync                Protocol     = "GraphSync"
	Success                  ResultStatus = "success"
	QueryFailure             ResultStatus = "query_failure"
	QueryResponseUnavailable ResultStatus = "query_response_unavailable"
	QueryResponseError       ResultStatus = "query_response_error"
	ProposalFailure          ResultStatus = "proposal_failure"
	RetrieveFailure          ResultStatus = "retrieve_failure"
	DataTransferFailure      ResultStatus = "data_transfer_failure"
	RetrieveTimeout          ResultStatus = "retrieve_timeout"
)

type TaskDefinition struct {
	ProtocolPreference []Protocol `json:"protocolPreference"`
	DataCids           []string   `json:"dataCids"`
	PieceCids          []string   `json:"pieceCids"`
	FromClients        []string   `json:"fromClients"`
}

type Input struct {
	ProtocolPreference []Protocol `json:"protocolPreference"`
	DataCid            string     `json:"dataCid"`
	PieceCid           string     `json:"pieceCid"`
	Label              string     `json:"label"`
}

type ResultStatus string

type Result struct {
	AuditorErrors         []string        `json:"auditorErrors"`
	TotalBytesDownloaded  uint64          `json:"totalBytesDownloaded"`
	MaxAverageSpeedPerSec float64         `json:"maxAverageSpeedPerSec"`
	MinTimeToFirstByte    time.Duration   `json:"minTimeToFirstByte"`
	Results               []ResultContent `json:"results"`
}
