package api

import "time"

// TempReading is one temperature sensor value.
type TempReading struct {
	Key     string  `json:"key"`
	Chip    string  `json:"chip"`
	Label   string  `json:"label"`
	Celsius float64 `json:"celsius"`
	Warn    bool    `json:"warn"`
}

// HostSnapshot is the current host-level resource usage.
type HostSnapshot struct {
	CPUPct    float64   `json:"cpu_pct"`
	MemUsed   uint64    `json:"mem_used"`
	MemTotal  uint64    `json:"mem_total"`
	MemPct    float64   `json:"mem_pct"`
	Load1     float64   `json:"load1"`
	Load5     float64   `json:"load5"`
	Load15    float64   `json:"load15"`
	NetRxBps  float64   `json:"net_rx_bps"`
	NetTxBps  float64   `json:"net_tx_bps"`
	UpdatedAt time.Time `json:"updated_at"`
}

// ContainerSnapshot is one running container's current resource usage.
type ContainerSnapshot struct {
	Name     string  `json:"name"`
	ID       string  `json:"id"`
	Image    string  `json:"image"`
	CPUPct   float64 `json:"cpu_pct"`
	MemUsed  uint64  `json:"mem_used"`
	MemLimit uint64  `json:"mem_limit"`
	MemPct   float64 `json:"mem_pct"`
	NetRxBps float64 `json:"net_rx_bps"`
	NetTxBps float64 `json:"net_tx_bps"`
}

// VolumeSnapshot is the usage of one monitored filesystem.
type VolumeSnapshot struct {
	Name          string  `json:"name"`
	ContainerPath string  `json:"container_path"`
	Used          uint64  `json:"used"`
	Total         uint64  `json:"total"`
	UsedPct       float64 `json:"used_pct"`
}

// DiskSmart is one SSD SMART snapshot. Value fields are nil when the drive does
// not report the attribute.
type DiskSmart struct {
	Disk           string    `json:"disk"`
	Model          string    `json:"model"`
	Serial         string    `json:"serial"`
	PercentageUsed *int64    `json:"percentage_used,omitempty"`
	TBWBytes       *int64    `json:"tbw_bytes,omitempty"`
	PowerOnHours   *int64    `json:"power_on_hours,omitempty"`
	MediaErrors    *int64    `json:"media_errors,omitempty"`
	Reallocated    *int64    `json:"reallocated,omitempty"`
	TempC          *int64    `json:"temp_c,omitempty"`
	CollectedAt    time.Time `json:"collected_at"`
}

// CollectorStatus describes the health of one collector loop.
type CollectorStatus struct {
	LastSuccess time.Time `json:"last_success,omitempty"`
	LastError   string    `json:"last_error,omitempty"`
	DurationMS  int64     `json:"duration_ms"`
	Runs        int64     `json:"runs"`
}

// Overview is everything the dashboard needs for one refresh.
type Overview struct {
	TS         time.Time                  `json:"ts"`
	Temps      []TempReading              `json:"temps"`
	Host       *HostSnapshot              `json:"host,omitempty"`
	Containers []ContainerSnapshot        `json:"containers"`
	Volumes    []VolumeSnapshot           `json:"volumes"`
	Smart      []DiskSmart                `json:"smart"`
	Collectors map[string]CollectorStatus `json:"collectors"`
}
