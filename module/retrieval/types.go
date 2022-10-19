package retrieval

import "validation-bot/task"

type Protocol string

const (
	GraphSync Protocol = "GraphSync"
	BitSwap   Protocol = "BitSwap"
	HTTP      Protocol = "HTTP"
)

type Input struct {
	task.Task
	ProtocolPreference []Protocol `json:"protocolPreference"`
	DataCid            string     `json:"dataCid"`
	PieceCid           string     `json:"pieceCid"`
}

type ResultContent struct {
}
