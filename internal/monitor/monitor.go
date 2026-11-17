// Package monitor holds the application use cases: it turns collector snapshots
// into the dashboard read model and recorded samples, persists closed minute
// buckets, applies the wear policy and answers history queries. The ports it
// needs are declared here; internal/app wires the adapters in.
package monitor

import (
	"log/slog"
	"sync"
	"time"

	"mon/internal/api"
	"mon/internal/domain"
	"mon/internal/recorder"
)

// Store is the persistence port the monitor needs.
type Store interface {
	InsertMinute(points []domain.Point) error
	History(kind domain.Kind, names []string, from, to int64, res domain.Resolution) ([]api.SeriesHistory, error)
	LastWear(disk string) (*domain.WearObservation, error)
	AppendWear(row domain.WearObservation) error
	WearLatest() ([]domain.WearObservation, error)
	WearLog(limit int) ([]domain.WearObservation, error)
}

// Monitor is safe for concurrent use by collector loops and HTTP handlers.
type Monitor struct {
	store Store
	log   *slog.Logger
	rec   *recorder.Recorder

	mu   sync.RWMutex
	snap api.Overview
}

// New returns an empty monitor with non-nil slices so JSON never emits null.
func New(st Store, log *slog.Logger) *Monitor {
	if log == nil {
		log = slog.Default()
	}
	return &Monitor{
		store: st,
		log:   log,
		rec:   recorder.New(),
		snap: api.Overview{
			Temps:      []api.TempReading{},
			Containers: []api.ContainerSnapshot{},
			Volumes:    []api.VolumeSnapshot{},
			Smart:      []api.DiskSmart{},
			Collectors: map[string]api.CollectorStatus{},
		},
	}
}

// Snapshot returns a deep copy of the current read model.
func (m *Monitor) Snapshot() api.Overview {
	m.mu.RLock()
	defer m.mu.RUnlock()
	out := m.snap
	out.Temps = append([]api.TempReading{}, m.snap.Temps...)
	out.Containers = append([]api.ContainerSnapshot{}, m.snap.Containers...)
	out.Volumes = append([]api.VolumeSnapshot{}, m.snap.Volumes...)
	out.Smart = append([]api.DiskSmart{}, m.snap.Smart...)
	out.Collectors = make(map[string]api.CollectorStatus, len(m.snap.Collectors))
	for k, v := range m.snap.Collectors {
		out.Collectors[k] = v
	}
	if m.snap.Host != nil {
		h := *m.snap.Host
		out.Host = &h
	}
	return out
}

// SetCollector records the outcome of one collector run.
func (m *Monitor) SetCollector(name string, at time.Time, err error, duration time.Duration) {
	m.mu.Lock()
	defer m.mu.Unlock()
	st := m.snap.Collectors[name]
	st.Runs++
	st.DurationMS = duration.Milliseconds()
	if err != nil {
		st.LastError = err.Error()
	} else {
		st.LastError = ""
		st.LastSuccess = at
	}
	m.snap.Collectors[name] = st
}

// Flush persists every fully closed minute bucket.
func (m *Monitor) Flush(now time.Time) error {
	points := m.rec.CollectCompleted(now)
	if len(points) == 0 {
		return nil
	}
	return m.store.InsertMinute(points)
}

// History returns the aggregated points of the requested series names.
func (m *Monitor) History(kind domain.Kind, names []string, from, to int64, res domain.Resolution) ([]api.SeriesHistory, error) {
	return m.store.History(kind, names, from, to, res)
}

// WearLatest returns the newest wear row per disk.
func (m *Monitor) WearLatest() ([]api.Wear, error) {
	rows, err := m.store.WearLatest()
	if err != nil {
		return nil, err
	}
	return wearRows(rows), nil
}

// WearLog returns the newest limit wear rows across all disks.
func (m *Monitor) WearLog(limit int) ([]api.Wear, error) {
	rows, err := m.store.WearLog(limit)
	if err != nil {
		return nil, err
	}
	return wearRows(rows), nil
}

func wearRows(rows []domain.WearObservation) []api.Wear {
	if rows == nil {
		return nil
	}
	out := make([]api.Wear, len(rows))
	for i, r := range rows {
		out[i] = api.Wear{
			Disk: r.Disk, TS: r.TS, Model: r.Model, Serial: r.Serial,
			PercentageUsed: r.PercentageUsed, TBWBytes: r.TBWBytes,
			PowerOnHours: r.PowerOnHours, MediaErrors: r.MediaErrors,
			Reallocated: r.Reallocated, TempC: r.TempC,
			Reason: domain.FormatReasons(r.Reasons),
		}
	}
	return out
}
