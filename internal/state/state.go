// Package state keeps the newest successful snapshot of every collector for
// the HTTP API. Collectors only write through the setters; readers get a copy.
package state

import (
	"sync"
	"time"

	"mon/internal/collect"
)

// CollectorStatus describes the health of one collector loop.
type CollectorStatus struct {
	LastSuccess time.Time `json:"last_success,omitempty"`
	LastError   string    `json:"last_error,omitempty"`
	DurationMS  int64     `json:"duration_ms"`
	Runs        int64     `json:"runs"`
}

// Snapshot is everything the dashboard needs for one refresh.
type Snapshot struct {
	TS         time.Time                   `json:"ts"`
	Temps      []collect.TempReading       `json:"temps"`
	Host       *collect.HostSnapshot       `json:"host,omitempty"`
	Containers []collect.ContainerSnapshot `json:"containers"`
	Volumes    []collect.VolumeSnapshot    `json:"volumes"`
	Smart      []collect.DiskSmart         `json:"smart"`
	Collectors map[string]CollectorStatus  `json:"collectors"`
}

// State is safe for concurrent use.
type State struct {
	mu   sync.RWMutex
	snap Snapshot
}

// New returns an empty state with non-nil slices so JSON never emits null.
func New() *State {
	return &State{snap: Snapshot{
		Temps:      []collect.TempReading{},
		Containers: []collect.ContainerSnapshot{},
		Volumes:    []collect.VolumeSnapshot{},
		Smart:      []collect.DiskSmart{},
		Collectors: map[string]CollectorStatus{},
	}}
}

// SetTemps replaces the temperature snapshot.
func (s *State) SetTemps(ts time.Time, temps []collect.TempReading) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.snap.TS = ts
	s.snap.Temps = append([]collect.TempReading{}, temps...)
}

// SetHost replaces the host snapshot.
func (s *State) SetHost(ts time.Time, h collect.HostSnapshot) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.snap.TS = ts
	s.snap.Host = &h
}

// SetContainers replaces the container list.
func (s *State) SetContainers(ts time.Time, cs []collect.ContainerSnapshot) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.snap.TS = ts
	s.snap.Containers = append([]collect.ContainerSnapshot{}, cs...)
}

// SetVolumes replaces the filesystem list.
func (s *State) SetVolumes(ts time.Time, vs []collect.VolumeSnapshot) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.snap.TS = ts
	s.snap.Volumes = append([]collect.VolumeSnapshot{}, vs...)
}

// SetSmart replaces the SMART list.
func (s *State) SetSmart(ts time.Time, ds []collect.DiskSmart) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.snap.TS = ts
	s.snap.Smart = append([]collect.DiskSmart{}, ds...)
}

// SetCollector records the outcome of one collector run.
func (s *State) SetCollector(name string, at time.Time, err error, duration time.Duration) {
	s.mu.Lock()
	defer s.mu.Unlock()
	st := s.snap.Collectors[name]
	st.Runs++
	st.DurationMS = duration.Milliseconds()
	if err != nil {
		st.LastError = err.Error()
	} else {
		st.LastError = ""
		st.LastSuccess = at
	}
	s.snap.Collectors[name] = st
}

// Snapshot returns a deep copy of the current state.
func (s *State) Snapshot() Snapshot {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := s.snap
	out.Temps = append([]collect.TempReading{}, s.snap.Temps...)
	out.Containers = append([]collect.ContainerSnapshot{}, s.snap.Containers...)
	out.Volumes = append([]collect.VolumeSnapshot{}, s.snap.Volumes...)
	out.Smart = append([]collect.DiskSmart{}, s.snap.Smart...)
	out.Collectors = make(map[string]CollectorStatus, len(s.snap.Collectors))
	for k, v := range s.snap.Collectors {
		out.Collectors[k] = v
	}
	if s.snap.Host != nil {
		h := *s.snap.Host
		out.Host = &h
	}
	return out
}
