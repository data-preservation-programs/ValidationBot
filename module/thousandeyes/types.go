package thousandeyes

import (
	"validation-bot/task"
)

type Input struct {
	task.Task
}

type MultiAddrStr string

type ResultContent struct {
	Metrics      []Metric `json:"metrics"`
	MinLatency   float64  `json:"minLatency"`
	ClosestAgent Agent    `json:"closestAgent"`
}

type ResultMap = map[MultiAddrStr]ResultContent

type Agent struct {
	AgentID   int32  `json:"agentId"`
	AgentName string `json:"agentName"`
	Location  string `json:"location"`
	CountryID int32  `json:"countryId"`
}

type AgentID struct {
	AgentID int32 `json:"agentId"`
}

type InvokeInstantTestRequest struct {
	Port     int       `json:"port"`
	Server   string    `json:"server"`
	Protocol string    `json:"protocol"`
	Agents   []AgentID `json:"agents"`
}

type InstantTest struct {
	TestID int `json:"testId"`
}

type Net struct {
	Metrics []Metric `json:"metrics"`
}

type Metric struct {
	AgentName  string  `json:"agentName"`
	CountryID  string  `json:"countryId"`
	Date       string  `json:"date"`
	ServerIP   string  `json:"serverIp"`
	Loss       float64 `json:"loss"`
	PermaLink  string  `json:"permaLink"`
	AgentID    int32   `json:"agentId"`
	Server     string  `json:"server"`
	RoundID    int64   `json:"roundId"`
	MinLatency float64 `json:"minLatency"`
	MaxLatency float64 `json:"maxLatency"`
	AvgLatency float64 `json:"avgLatency"`
	Jitter     float64 `json:"jitter"`
}

type InvokeInstantTestResponse struct {
	Test []InstantTest `json:"test"`
}

type RetrieveTestResultResponse struct {
	Net Net `json:"net"`
}
