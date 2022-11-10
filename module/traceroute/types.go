package traceroute

type Hop struct {
	Hop    int     `json:"hop"`
	Probes []Probe `json:"probes"`
}

type Probe struct {
	IP   string  `json:"ip"`
	Name string  `json:"name"`
	RTT  float64 `json:"rtt"`
}

//nolint:tagliatelle
type Output struct {
	DestinationIP   string `json:"destination_ip"`
	DestinationName string `json:"destination_name"`
	Hops            []Hop  `json:"hops"`
}

type Status string

const (
	Success             Status = "success"
	InvalidMultiAddress Status = "invalid_multi_address"
	CannotConnect       Status = "cannot_connect"
	NoMultiAddress      Status = "no_multi_address"
)

type HopResult struct {
	LastHopOverheadMs float64 `json:"lastHopOverheadMs,omitempty"`
	Hops              []Hop   `json:"hops"`
}

type ResultContent struct {
	Status       Status               `json:"status"`
	ErrorMessage string               `json:"errorMessage,omitempty"`
	Traces       map[string]HopResult `json:"traces,omitempty"`
}
