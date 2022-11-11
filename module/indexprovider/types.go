package indexprovider

type Status string

const (
	Enabled             Status = "enabled"
	Disabled            Status = "disabled"
	CannotConnect       Status = "cannot_connect"
	InvalidMultiAddress Status = "invalid_multi_address"
	NoMultiAddress      Status = "no_multi_address"
)

type ResultContent struct {
	Status       Status `json:"status"`
	ErrorMessage string `json:"errorMessage,omitempty"`
	RootCid      string `json:"rootCid,omitempty"`
}
