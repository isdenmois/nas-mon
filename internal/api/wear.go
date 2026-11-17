package api

// Wear is one SSD wear snapshot in the wear log.
type Wear struct {
	Disk           string `json:"disk"`
	TS             int64  `json:"ts"`
	Model          string `json:"model,omitempty"`
	Serial         string `json:"serial,omitempty"`
	PercentageUsed *int64 `json:"percentage_used,omitempty"`
	TBWBytes       *int64 `json:"tbw_bytes,omitempty"`
	PowerOnHours   *int64 `json:"power_on_hours,omitempty"`
	MediaErrors    *int64 `json:"media_errors,omitempty"`
	Reallocated    *int64 `json:"reallocated,omitempty"`
	TempC          *int64 `json:"temp_c,omitempty"`
	Reason         string `json:"reason,omitempty"`
}
