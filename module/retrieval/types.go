package retrieval

import (
	"time"
)

type Protocol string

const (
	GraphSync                Protocol     = "GraphSync"
	Http                     Protocol     = "http"
	Https                    Protocol     = "https"
	Success                  ResultStatus = "success"
	QueryFailure             ResultStatus = "query_failure"
	QueryResponseUnavailable ResultStatus = "query_response_unavailable"
	QueryResponseError       ResultStatus = "query_response_error"
	ProposalFailure          ResultStatus = "proposal_failure"
	RetrieveFailure          ResultStatus = "retrieve_failure"
	DataTransferFailure      ResultStatus = "data_transfer_failure"
	RetrieveTimeout          ResultStatus = "retrieve_timeout"
	ProtocolUnsupported      ResultStatus = "protocol_unsupported"
)

type TaskDefinition struct {
	ProtocolPreference []Protocol `json:"protocolPreference"`
	DataCids           []string   `json:"dataCids"`
	PieceCids          []string   `json:"pieceCids"`
	FromClients        []string   `json:"fromClients"`
}

type Input struct {
	ProtocolPreference []Protocol `json:"protocolPreference"`
	DataCid            string     `json:"dataCid,omitempty"`
	PieceCid           string     `json:"pieceCid,omitempty"`
	Label              string     `json:"label,omitempty"`
	DealID             string     `json:"dealId,omitempty"`
	Client             string     `json:"client,omitempty"`
}

type ResultStatus string

type Result struct {
	Status                ResultStatus               `json:"status"`
	ErrorMessage          string                     `json:"errorMessage,omitempty"`
	TotalBytesDownloaded  uint64                     `json:"totalBytesDownloaded"`
	MaxAverageSpeedPerSec float64                    `json:"maxAverageSpeedPerSec"`
	MinTimeToFirstByte    time.Duration              `json:"minTimeToFirstByte"`
	Results               map[Protocol]ResultContent `json:"results"`
}
