package queryask

type QueryStatus string

const (
	Success             QueryStatus = "success"
	InvalidProviderID   QueryStatus = "invalid_provider_id"
	NoPeerID            QueryStatus = "no_peer_id"
	InvalidMultiAddress QueryStatus = "invalid_multi_address"
	CannotConnect       QueryStatus = "cannot_connect"
	NoMultiAddress      QueryStatus = "no_multi_address"
	StreamFailure       QueryStatus = "stream_failure"
)

type ResultContent struct {
	PeerID        string      `json:"peerId,omitempty"`
	MultiAddrs    []string    `json:"multiAddrs,omitempty"`
	Status        QueryStatus `json:"status"`
	ErrorMessage  string      `json:"errorMessage,omitempty"`
	Price         string      `json:"price,omitempty"`
	VerifiedPrice string      `json:"verifiedPrice,omitempty"`
	MinPieceSize  uint64      `json:"minPieceSize,omitempty"`
	MaxPieceSize  uint64      `json:"maxPieceSize,omitempty"`
}
