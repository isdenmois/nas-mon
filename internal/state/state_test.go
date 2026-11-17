package state

import (
	"errors"
	"testing"
	"time"

	"mon/internal/collect"
)

func TestSnapshotIsACopy(t *testing.T) {
	s := New()
	now := time.Unix(100, 0)
	s.SetTemps(now, []collect.TempReading{{Key: "cpu", Celsius: 40}})
	s.SetContainers(now, []collect.ContainerSnapshot{{Name: "web"}})
	s.SetHost(now, collect.HostSnapshot{CPUPct: 5})
	s.SetCollector("temp", now, errors.New("boom"), 3*time.Millisecond)

	snap := s.Snapshot()
	snap.Temps[0].Celsius = 99
	snap.Containers[0].Name = "mutated"
	snap.Host.CPUPct = 99
	snap.Collectors["temp"] = CollectorStatus{}

	fresh := s.Snapshot()
	if fresh.Temps[0].Celsius != 40 || fresh.Containers[0].Name != "web" ||
		fresh.Host.CPUPct != 5 || fresh.Collectors["temp"].LastError != "boom" {
		t.Errorf("state mutated through snapshot: %+v", fresh)
	}
	if fresh.Collectors["temp"].Runs != 1 || fresh.Collectors["temp"].DurationMS != 3 {
		t.Errorf("collector status = %+v", fresh.Collectors["temp"])
	}
	if !fresh.TS.Equal(now) {
		t.Errorf("ts = %v", fresh.TS)
	}
}

func TestCollectorRecoveryClearsError(t *testing.T) {
	s := New()
	at := time.Unix(100, 0)
	s.SetCollector("docker", at, errors.New("socket gone"), 0)
	s.SetCollector("docker", at.Add(time.Second), nil, 0)
	st := s.Snapshot().Collectors["docker"]
	if st.LastError != "" || !st.LastSuccess.Equal(at.Add(time.Second)) || st.Runs != 2 {
		t.Errorf("status = %+v", st)
	}
}
