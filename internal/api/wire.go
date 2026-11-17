// Package api is the JSON contract of the HTTP API. It mirrors
// web/src/shared/api/contracts.ts: a field added here must be added there in the
// same commit.
package api

// Step is the aggregation resolution returned with a history query.
type Step string

const (
	StepMinute Step = "minute"
	StepHour   Step = "hour"
	StepDay    Step = "day"
)

// SeriesPoint is one aggregated bucket in a history response.
type SeriesPoint struct {
	TS  int64   `json:"ts"`
	Avg float64 `json:"avg"`
	Min float64 `json:"min"`
	Max float64 `json:"max"`
	N   int64   `json:"n"`
}

// SeriesHistory is the history of one series.
type SeriesHistory struct {
	Kind   string        `json:"kind"`
	Name   string        `json:"name"`
	Points []SeriesPoint `json:"points"`
}
