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

// SeriesPoint is one aggregated bucket in a history response. Only what the
// charts draw crosses the wire: min, max and n stay in the database because
// rollups need them, but sending them tripled the payload for nobody.
type SeriesPoint struct {
	TS  int64   `json:"ts"`
	Avg float64 `json:"avg"`
}

// SeriesHistory is the history of one series. Kind and name are informational:
// clients address history by ID.
type SeriesHistory struct {
	ID     int64         `json:"id"`
	Kind   string        `json:"kind"`
	Name   string        `json:"name"`
	Points []SeriesPoint `json:"points"`
}

// SeriesDescriptor identifies a series that has been recorded at least once.
type SeriesDescriptor struct {
	ID   int64  `json:"id"`
	Kind string `json:"kind"`
	Name string `json:"name"`
}
