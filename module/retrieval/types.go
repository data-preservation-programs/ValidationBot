package retrieval

import (
	"context"
	"time"

	"github.com/filecoin-project/go-address"
	"github.com/ipfs/go-cid"
)

type Protocol string

type Retriever interface {
	Retrieve(
		parent context.Context,
		minerAddress address.Address,
		dataCid cid.Cid,
		timeout time.Duration,
	) (*ResultContent, error)
}

type RetrieverBuilder interface {
	Build() (Retriever, Cleanup, error)
}

const (
	Bitswap                  Protocol     = "Bitswap"
	GraphSync                Protocol     = "GraphSync"
	BlockParseException      ResultStatus = "block_parse_exception"
	BlockReceived            ResultStatus = "block_received"
	RetrieveComplete         ResultStatus = "retrieve_complete"
	FirstByteReceived        ResultStatus = "first_byte_received"
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

type TimeEventPair struct {
	Timestamp time.Time `json:"timestamp"`
	Code      string    `json:"code"`
	Message   string    `json:"message"`
	Received  uint64    `json:"received"`
}

type CalculatedStats struct {
	Events             []TimeEventPair `json:"retrievalEvents,omitempty"`
	BytesDownloaded    uint64          `json:"bytesDownloaded,omitempty"`
	AverageSpeedPerSec float64         `json:"averageSpeedPerSec,omitempty"`
	TimeElapsed        time.Duration   `json:"timeElapsed,omitempty"`
	TimeToFirstByte    time.Duration   `json:"timeToFirstByte,omitempty"`
}

type ResultContent struct {
	Status       ResultStatus `json:"status"`
	ErrorMessage string       `json:"errorMessage,omitempty"`
	Protocol     Protocol     `json:"protocol"`
	CalculatedStats
}
