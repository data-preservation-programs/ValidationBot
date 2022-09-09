package model

type ValidationInput struct {
	Type               string `json:"type"`
	Cid                string `json:"cid"`
	Provider           string `json:"provider"`
	MaxDurationSeconds uint64 `json:"max_duration_seconds"`
	MaxDownloadBytes   uint64 `json:"max_download_bytes"`
}

type ValidationResult struct {
	Success           bool      `json:"success"`
	TimeToFirstByteMs int64     `json:"time_to_first_byte_ms"`
	BytesDownloaded   uint64    `json:"bytes_downloaded"`
	AverageSpeedBps   float64   `json:"average_speed_bps"`
	SpeedBpsP1        float64   `json:"speed_bps_p1"`
	SpeedBpsP5        float64   `json:"speed_bps_p5"`
	SpeedBpsP10       float64   `json:"speed_bps_p10"`
	SpeedBpsP25       float64   `json:"speed_bps_p25"`
	SpeedBpsP50       float64   `json:"speed_bps_p50"`
	SpeedBpsP75       float64   `json:"speed_bps_p75"`
	SpeedBpsP90       float64   `json:"speed_bps_p90"`
	SpeedBpsP95       float64   `json:"speed_bps_p95"`
	SpeedBpsP99       float64   `json:"speed_bps_p99"`
	ErrorCode         ErrorCode `json:"error_code"`
	ErrorMessage      string    `json:"error_message"`
}
