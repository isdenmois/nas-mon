package monitor

import (
	"errors"
	"io"
	"log/slog"
	"testing"
	"time"

	"mon/internal/api"
	"mon/internal/collect"
	"mon/internal/domain"
)

type fakeStore struct {
	inserted  [][]domain.Point
	series    []domain.Series
	history   []api.SeriesHistory
	histIDs   []int64
	histRes   domain.Resolution
	wear      []domain.WearObservation
	wearSince int64
	appended  []domain.WearObservation
	last      map[string]*domain.WearObservation
	err       error
}

func (f *fakeStore) InsertMinute(p []domain.Point) error {
	f.inserted = append(f.inserted, p)
	return f.err
}

func (f *fakeStore) ListSeries() ([]domain.Series, error) { return f.series, f.err }

func (f *fakeStore) Series(ids []int64) ([]domain.Series, error) {
	var out []domain.Series
	for _, id := range ids {
		for _, s := range f.series {
			if s.ID == id {
				out = append(out, s)
			}
		}
	}
	return out, f.err
}

func (f *fakeStore) History(ids []int64, _, _ int64, res domain.Resolution) ([]api.SeriesHistory, error) {
	f.histIDs, f.histRes = ids, res
	return f.history, f.err
}

func (f *fakeStore) LastWear(disk string) (*domain.WearObservation, error) {
	return f.last[disk], f.err
}

func (f *fakeStore) AppendWear(w domain.WearObservation) error {
	f.appended = append(f.appended, w)
	return f.err
}

func (f *fakeStore) WearLatest() ([]domain.WearObservation, error) { return f.wear, f.err }

func (f *fakeStore) WearLog(_ int, since int64) ([]domain.WearObservation, error) {
	f.wearSince = since
	return f.wear, f.err
}

func newMonitor(t *testing.T, st Store) *Monitor {
	t.Helper()
	return New(st, slog.New(slog.NewTextHandler(io.Discard, nil)))
}

func TestSnapshotIsACopy(t *testing.T) {
	m := newMonitor(t, &fakeStore{})
	now := time.Unix(100, 0)
	m.IngestTemps(now, []collect.TempReading{{Key: "cpu", Celsius: 40}})
	m.IngestContainers(now, []collect.ContainerSnapshot{{Name: "web"}})
	m.IngestHost(now, collect.HostSnapshot{CPUPct: 5})
	m.SetCollector("temp", now, errors.New("boom"), 3*time.Millisecond)

	snap := m.Snapshot()
	snap.Temps[0].Celsius = 99
	snap.Containers[0].Name = "mutated"
	snap.Host.CPUPct = 99
	snap.Collectors["temp"] = api.CollectorStatus{}

	fresh := m.Snapshot()
	if fresh.Temps[0].Celsius != 40 || fresh.Containers[0].Name != "web" ||
		fresh.Host.CPUPct != 5 || fresh.Collectors["temp"].LastError != "boom" {
		t.Errorf("read model mutated through snapshot: %+v", fresh)
	}
	if fresh.Collectors["temp"].Runs != 1 || fresh.Collectors["temp"].DurationMS != 3 {
		t.Errorf("collector status = %+v", fresh.Collectors["temp"])
	}
	if !fresh.TS.Equal(now) {
		t.Errorf("ts = %v", fresh.TS)
	}
}

func TestCollectorRecoveryClearsError(t *testing.T) {
	m := newMonitor(t, &fakeStore{})
	at := time.Unix(100, 0)
	m.SetCollector("docker", at, errors.New("socket gone"), 0)
	m.SetCollector("docker", at.Add(time.Second), nil, 0)
	st := m.Snapshot().Collectors["docker"]
	if st.LastError != "" || !st.LastSuccess.Equal(at.Add(time.Second)) || st.Runs != 2 {
		t.Errorf("status = %+v", st)
	}
}

func TestFlushWritesClosedMinutesOnlyOnce(t *testing.T) {
	st := &fakeStore{}
	m := newMonitor(t, st)
	now := time.Date(2026, 3, 4, 12, 0, 30, 0, time.UTC)

	m.IngestTemps(now.Add(-2*time.Minute), []collect.TempReading{{Key: "cpu", Celsius: 40}})
	m.IngestTemps(now.Add(-2*time.Minute), []collect.TempReading{{Key: "cpu", Celsius: 50}})
	m.IngestTemps(now, []collect.TempReading{{Key: "cpu", Celsius: 99}}) // still open

	if err := m.Flush(now); err != nil {
		t.Fatal(err)
	}
	if len(st.inserted) != 1 || len(st.inserted[0]) != 1 {
		t.Fatalf("inserted = %+v", st.inserted)
	}
	if p := st.inserted[0][0]; p.Avg != 45 || p.N != 2 || p.Kind != domain.Temp {
		t.Errorf("point = %+v, want avg 45 n 2", p)
	}
	if err := m.Flush(now); err != nil {
		t.Fatal(err)
	}
	if len(st.inserted) != 1 {
		t.Errorf("second flush inserted again: %+v", st.inserted)
	}
}

func TestIngestHostRecordsEveryKind(t *testing.T) {
	st := &fakeStore{}
	m := newMonitor(t, st)
	at := time.Date(2026, 3, 4, 12, 0, 0, 0, time.UTC)

	m.IngestHost(at, collect.HostSnapshot{CPUPct: 10, MemPct: 20, Load1: 1, Load5: 2, Load15: 3, NetRxBps: 4, NetTxBps: 5})
	if err := m.Flush(at.Add(time.Minute)); err != nil {
		t.Fatal(err)
	}
	kinds := map[domain.Kind]float64{}
	for _, p := range st.inserted[0] {
		if p.Name != "host" {
			t.Errorf("sample name = %q, want host", p.Name)
		}
		kinds[p.Kind] = p.Avg
	}
	want := map[domain.Kind]float64{
		domain.HostCPU: 10, domain.HostMem: 20, domain.HostLoad1: 1,
		domain.HostLoad5: 2, domain.HostLoad15: 3, domain.HostNetRx: 4, domain.HostNetTx: 5,
	}
	for kind, value := range want {
		if kinds[kind] != value {
			t.Errorf("%s = %v, want %v", kind, kinds[kind], value)
		}
	}
}

func TestIngestSmartAppliesWearPolicy(t *testing.T) {
	st := &fakeStore{last: map[string]*domain.WearObservation{}}
	m := newMonitor(t, st)
	at := time.Date(2026, 3, 4, 12, 0, 0, 0, time.UTC)
	disks := []collect.DiskSmart{{Disk: "nvme0n1", Model: "Samsung", Serial: "S1"}}

	if err := m.IngestSmart(at, disks); err != nil {
		t.Fatal(err)
	}
	if len(st.appended) != 1 {
		t.Fatalf("appended = %+v", st.appended)
	}
	if got := domain.FormatReasons(st.appended[0].Reasons); got != "first" {
		t.Errorf("reasons = %q, want first", got)
	}
	if len(m.Snapshot().Smart) != 1 {
		t.Errorf("smart snapshot = %+v", m.Snapshot().Smart)
	}

	prev := st.appended[0]
	st.last["nvme0n1"] = &prev
	if err := m.IngestSmart(at.Add(time.Hour), disks); err != nil {
		t.Fatal(err)
	}
	if len(st.appended) != 1 {
		t.Errorf("idle disk appended again: %+v", st.appended)
	}
}

func TestWearRowsUseTheWireContract(t *testing.T) {
	pct := int64(3)
	st := &fakeStore{wear: []domain.WearObservation{{
		Disk: "nvme0n1", TS: 100, Reasons: []domain.WearReason{domain.ReasonWear, domain.ReasonTBW},
		PercentageUsed: &pct,
	}}}
	m := newMonitor(t, st)
	rows, err := m.WearLog(10, 1234)
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) != 1 || rows[0].Reason != "wear+tbw" || *rows[0].PercentageUsed != 3 {
		t.Errorf("rows = %+v", rows)
	}
}

func TestHistoryResolvesSeriesIDs(t *testing.T) {
	st := &fakeStore{
		series:  []domain.Series{{ID: 7, Kind: domain.Temp, Name: "a"}},
		history: []api.SeriesHistory{{ID: 7, Kind: "temp", Name: "a"}},
	}
	m := newMonitor(t, st)
	got, err := m.History([]int64{7, 7}, 0, 60, domain.Hour)
	if err != nil || len(got) != 1 {
		t.Fatalf("history = %+v, %v", got, err)
	}
	if len(st.histIDs) != 1 || st.histIDs[0] != 7 || st.histRes != domain.Hour {
		t.Errorf("port called with %v %v", st.histIDs, st.histRes)
	}
}

func TestHistoryRejectsUnknownSeries(t *testing.T) {
	m := newMonitor(t, &fakeStore{series: []domain.Series{{ID: 7}}})
	if _, err := m.History([]int64{7, 99}, 0, 60, domain.Hour); !errors.Is(err, ErrUnknownSeries) {
		t.Errorf("err = %v, want ErrUnknownSeries", err)
	}
}

func TestOverviewIncludesSeriesRegistry(t *testing.T) {
	st := &fakeStore{series: []domain.Series{{ID: 3, Kind: domain.HostCPU, Name: "host"}}}
	m := newMonitor(t, st)
	ov, err := m.Overview()
	if err != nil {
		t.Fatal(err)
	}
	if len(ov.Series) != 1 || ov.Series[0].ID != 3 ||
		ov.Series[0].Kind != "host_cpu" || ov.Series[0].Name != "host" {
		t.Errorf("registry = %+v", ov.Series)
	}
	if ov.Series == nil {
		t.Error("registry is nil; JSON would emit null")
	}
}
