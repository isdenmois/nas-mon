// Package domain holds the metric vocabulary and the rules over it. Nothing in
// this package performs I/O, reads the clock, or knows about JSON or SQL.
package domain

// Kind identifies what a series measures. The strings are persisted in SQLite
// and used as Prometheus label values: treat them as permanent.
type Kind string

const (
	Temp Kind = "temp"

	HostCPU    Kind = "host_cpu"
	HostMem    Kind = "host_mem"
	HostLoad1  Kind = "host_load1"
	HostLoad5  Kind = "host_load5"
	HostLoad15 Kind = "host_load15"
	HostNetRx  Kind = "host_net_rx"
	HostNetTx  Kind = "host_net_tx"

	ContainerCPU    Kind = "container_cpu"
	ContainerMem    Kind = "container_mem"
	ContainerMemPct Kind = "container_mem_pct"
	ContainerNetRx  Kind = "container_net_rx"
	ContainerNetTx  Kind = "container_net_tx"

	VolumeUsedPct Kind = "volume_used_pct"
)

var kinds = []Kind{
	Temp,
	HostCPU, HostMem, HostLoad1, HostLoad5, HostLoad15, HostNetRx, HostNetTx,
	ContainerCPU, ContainerMem, ContainerMemPct, ContainerNetRx, ContainerNetTx,
	VolumeUsedPct,
}

// Kinds lists every kind the monitor records, in a stable order.
func Kinds() []Kind {
	return append([]Kind(nil), kinds...)
}

// Valid reports whether k is a known kind.
func (k Kind) Valid() bool {
	for _, known := range kinds {
		if k == known {
			return true
		}
	}
	return false
}
