// Package model holds the metric vocabulary shared by collectors, storage and
// the HTTP API. Names are stable: they are persisted in SQLite and used as
// Prometheus label values.
package model

const (
	KindTemp       = "temp"
	KindHostCPU    = "host_cpu"
	KindHostMem    = "host_mem"
	KindHostLoad1  = "host_load1"
	KindHostLoad5  = "host_load5"
	KindHostLoad15 = "host_load15"
	KindHostNetRx  = "host_net_rx"
	KindHostNetTx  = "host_net_tx"

	KindContainerCPU    = "container_cpu"
	KindContainerMem    = "container_mem"
	KindContainerMemPct = "container_mem_pct"
	KindContainerNetRx  = "container_net_rx"
	KindContainerNetTx  = "container_net_tx"

	KindVolumeUsedPct = "volume_used_pct"
)

// Sample is one observation fed into the recorder.
type Sample struct {
	Kind  string
	Name  string
	Value float64
}

// Point is one aggregated minute bucket ready for storage.
type Point struct {
	Kind string
	Name string
	TS   int64 // unix seconds, aligned to the start of the minute
	Avg  float64
	Min  float64
	Max  float64
	N    int64
}

// SeriesPoint is one aggregated point returned by history queries.
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
