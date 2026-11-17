package monitor

import (
	"time"

	"mon/internal/api"
	"mon/internal/collect"
	"mon/internal/domain"
)

// IngestTemps replaces the temperature read model and records one sample per
// sensor. The mapping from a collector snapshot to kinds and samples lives here
// and nowhere else.
func (m *Monitor) IngestTemps(at time.Time, temps []collect.TempReading) {
	readings := make([]api.TempReading, 0, len(temps))
	for _, t := range temps {
		readings = append(readings, api.TempReading{
			Key: t.Key, Chip: t.Chip, Label: t.Label, Celsius: t.Celsius, Warn: t.Warn,
		})
		m.rec.AddAt(domain.Temp, t.Key, t.Celsius, at)
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	m.snap.TS = at
	m.snap.Temps = readings
}

// IngestHost replaces the host read model and records its samples.
func (m *Monitor) IngestHost(at time.Time, h collect.HostSnapshot) {
	m.rec.AddAt(domain.HostCPU, "host", h.CPUPct, at)
	m.rec.AddAt(domain.HostMem, "host", h.MemPct, at)
	m.rec.AddAt(domain.HostLoad1, "host", h.Load1, at)
	m.rec.AddAt(domain.HostLoad5, "host", h.Load5, at)
	m.rec.AddAt(domain.HostLoad15, "host", h.Load15, at)
	m.rec.AddAt(domain.HostNetRx, "host", h.NetRxBps, at)
	m.rec.AddAt(domain.HostNetTx, "host", h.NetTxBps, at)

	snap := api.HostSnapshot{
		CPUPct: h.CPUPct, MemUsed: h.MemUsed, MemTotal: h.MemTotal, MemPct: h.MemPct,
		Load1: h.Load1, Load5: h.Load5, Load15: h.Load15,
		NetRxBps: h.NetRxBps, NetTxBps: h.NetTxBps, UpdatedAt: h.UpdatedAt,
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	m.snap.TS = at
	m.snap.Host = &snap
}

// IngestContainers replaces the container read model and records its samples.
func (m *Monitor) IngestContainers(at time.Time, cs []collect.ContainerSnapshot) {
	out := make([]api.ContainerSnapshot, 0, len(cs))
	for _, c := range cs {
		out = append(out, api.ContainerSnapshot{
			Name: c.Name, ID: c.ID, Image: c.Image, CPUPct: c.CPUPct,
			MemUsed: c.MemUsed, MemLimit: c.MemLimit, MemPct: c.MemPct,
			NetRxBps: c.NetRxBps, NetTxBps: c.NetTxBps,
		})
		m.rec.AddAt(domain.ContainerCPU, c.Name, c.CPUPct, at)
		m.rec.AddAt(domain.ContainerMem, c.Name, float64(c.MemUsed), at)
		m.rec.AddAt(domain.ContainerMemPct, c.Name, c.MemPct, at)
		m.rec.AddAt(domain.ContainerNetRx, c.Name, c.NetRxBps, at)
		m.rec.AddAt(domain.ContainerNetTx, c.Name, c.NetTxBps, at)
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	m.snap.TS = at
	m.snap.Containers = out
}

// IngestVolumes replaces the filesystem read model and records its samples.
func (m *Monitor) IngestVolumes(at time.Time, vs []collect.VolumeSnapshot) {
	out := make([]api.VolumeSnapshot, 0, len(vs))
	for _, v := range vs {
		out = append(out, api.VolumeSnapshot{
			Name: v.Name, ContainerPath: v.ContainerPath,
			Used: v.Used, Total: v.Total, UsedPct: v.UsedPct,
		})
		m.rec.AddAt(domain.VolumeUsedPct, v.Name, v.UsedPct, at)
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	m.snap.TS = at
	m.snap.Volumes = out
}

// IngestSmart replaces the SMART read model and appends wear-log rows that pass
// the domain policy.
func (m *Monitor) IngestSmart(at time.Time, disks []collect.DiskSmart) error {
	out := make([]api.DiskSmart, 0, len(disks))
	for _, d := range disks {
		out = append(out, api.DiskSmart{
			Disk: d.Disk, Model: d.Model, Serial: d.Serial,
			PercentageUsed: d.PercentageUsed, TBWBytes: d.TBWBytes,
			PowerOnHours: d.PowerOnHours, MediaErrors: d.MediaErrors,
			Reallocated: d.Reallocated, TempC: d.TempC, CollectedAt: d.CollectedAt,
		})
	}
	m.mu.Lock()
	m.snap.TS = at
	m.snap.Smart = out
	m.mu.Unlock()
	return m.recordWear(at, disks)
}
