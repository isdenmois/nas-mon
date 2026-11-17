package httpapi

import (
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"mon/internal/api"
	"mon/internal/domain"
	"mon/internal/monitor"
)

type historyCall struct {
	ids  []int64
	from int64
	to   int64
	step domain.Resolution
}

type fakeMonitor struct {
	overview  api.Overview
	histories []api.SeriesHistory
	latest    []api.Wear
	log       []api.Wear
	calls     []historyCall
	wearSince int64
	err       error
}

func (f *fakeMonitor) Snapshot() api.Overview { return f.overview }

func (f *fakeMonitor) Overview() (api.Overview, error) { return f.overview, f.err }

func (f *fakeMonitor) History(ids []int64, from, to int64, res domain.Resolution) ([]api.SeriesHistory, error) {
	f.calls = append(f.calls, historyCall{ids, from, to, res})
	return f.histories, f.err
}

func (f *fakeMonitor) WearLatest() ([]api.Wear, error) { return f.latest, f.err }

func (f *fakeMonitor) WearLog(_ int, since int64) ([]api.Wear, error) {
	f.wearSince = since
	return f.log, f.err
}

func testServer(t *testing.T, mon Monitor) (*Server, http.Handler) {
	t.Helper()
	s := New(mon, slog.New(slog.NewTextHandler(testWriter{t}, nil)))
	return s, s.Handler()
}

type testWriter struct{ t *testing.T }

func (w testWriter) Write(p []byte) (int, error) {
	w.t.Log(strings.TrimSpace(string(p)))
	return len(p), nil
}

func get(h http.Handler, path string) *httptest.ResponseRecorder {
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, path, nil))
	return rec
}

func decode[T any](t *testing.T, rec *httptest.ResponseRecorder) T {
	t.Helper()
	var out T
	if err := json.Unmarshal(rec.Body.Bytes(), &out); err != nil {
		t.Fatalf("decode %q: %v", rec.Body.String(), err)
	}
	return out
}

func TestOverview(t *testing.T) {
	at := time.Date(2026, 1, 1, 10, 0, 0, 0, time.UTC)
	mon := &fakeMonitor{overview: api.Overview{
		TS:         at,
		Temps:      []api.TempReading{{Key: "k10temp:Tctl", Chip: "k10temp", Label: "Tctl", Celsius: 42}},
		Host:       &api.HostSnapshot{CPUPct: 12.5, MemUsed: 100, MemTotal: 200, MemPct: 50},
		Containers: []api.ContainerSnapshot{{Name: "web", CPUPct: 5}},
		Volumes:    []api.VolumeSnapshot{{Name: "/", UsedPct: 42}},
		Smart:      []api.DiskSmart{{Disk: "nvme0n1", Model: "Samsung"}},
		Collectors: map[string]api.CollectorStatus{"temp": {LastSuccess: at, Runs: 1}},
		Series:     []api.SeriesDescriptor{{ID: 3, Kind: "host_cpu", Name: "host"}},
	}}

	_, h := testServer(t, mon)
	rec := get(h, "/api/overview")
	if rec.Code != 200 {
		t.Fatalf("status = %d: %s", rec.Code, rec.Body)
	}
	got := decode[api.Overview](t, rec)
	if len(got.Temps) != 1 || got.Temps[0].Key != "k10temp:Tctl" {
		t.Errorf("temps = %+v", got.Temps)
	}
	if got.Host == nil || got.Host.CPUPct != 12.5 {
		t.Errorf("host = %+v", got.Host)
	}
	if len(got.Containers) != 1 || got.Containers[0].Name != "web" {
		t.Errorf("containers = %+v", got.Containers)
	}
	if got.Collectors["temp"].LastSuccess.IsZero() {
		t.Errorf("collector = %+v", got.Collectors)
	}
	if len(got.Series) != 1 || got.Series[0].ID != 3 {
		t.Errorf("series registry = %+v", got.Series)
	}
}

func TestHistoryRangeAndGrouping(t *testing.T) {
	now := time.Date(2026, 3, 4, 12, 34, 56, 0, time.UTC)
	mon := &fakeMonitor{histories: []api.SeriesHistory{
		{ID: 7, Kind: "temp", Name: "a", Points: []api.SeriesPoint{{TS: 1, Avg: 40}}},
		{ID: 9, Kind: "host_cpu", Name: "host"},
	}}
	s, h := testServer(t, mon)
	s.now = func() time.Time { return now }

	rec := get(h, "/api/history?range=7d&id=7&id=8&id=9")
	if rec.Code != 200 {
		t.Fatalf("status = %d: %s", rec.Code, rec.Body)
	}
	if len(mon.calls) != 1 {
		t.Fatalf("calls = %+v", mon.calls)
	}
	call := mon.calls[0]
	if len(call.ids) != 3 || call.ids[0] != 7 || call.ids[2] != 9 || call.step != domain.Hour {
		t.Errorf("call = %+v", call)
	}
	wantFrom := now.Add(-7 * 24 * time.Hour).Unix()
	wantFrom -= wantFrom % 3600
	if call.from != wantFrom || call.to != now.Unix() {
		t.Errorf("window = %d..%d, want %d..%d", call.from, call.to, wantFrom, now.Unix())
	}

	resp := decode[struct {
		Step   string              `json:"step"`
		Series []api.SeriesHistory `json:"series"`
	}](t, rec)
	if resp.Step != "hour" || len(resp.Series) != 2 || resp.Series[0].ID != 7 ||
		resp.Series[0].Points[0].Avg != 40 {
		t.Errorf("response = %+v", resp)
	}
}

func TestHistoryUnknownSeries(t *testing.T) {
	mon := &fakeMonitor{err: monitor.ErrUnknownSeries}
	_, h := testServer(t, mon)
	if rec := get(h, "/api/history?range=1h&id=99"); rec.Code != http.StatusNotFound {
		t.Errorf("status = %d, want 404 (%s)", rec.Code, rec.Body)
	}
}

func TestHistoryRejectsBadInput(t *testing.T) {
	_, h := testServer(t, &fakeMonitor{})
	cases := []string{
		"/api/history?range=24h",
		"/api/history?range=24h&id=oops",
		"/api/history?range=14d&id=1",
		"/api/history?range=24h&id=0",
		"/api/history?range=24h&id=-3",
	}
	for _, path := range cases {
		if rec := get(h, path); rec.Code != http.StatusBadRequest {
			t.Errorf("%s: status = %d, want 400 (%s)", path, rec.Code, rec.Body)
		}
	}
}

func TestHistoryMonitorError(t *testing.T) {
	mon := &fakeMonitor{err: errors.New("db down")}
	_, h := testServer(t, mon)
	if rec := get(h, "/api/history?range=1h&id=1"); rec.Code != 500 {
		t.Errorf("status = %d", rec.Code)
	}
}

func TestWear(t *testing.T) {
	pct := int64(3)
	mon := &fakeMonitor{
		latest: []api.Wear{{Disk: "nvme0n1", PercentageUsed: &pct}},
		log:    []api.Wear{{Disk: "nvme0n1", TS: 100, Reason: "daily"}},
	}
	_, h := testServer(t, mon)
	rec := get(h, "/api/wear?limit=5")
	if rec.Code != 200 {
		t.Fatalf("status = %d", rec.Code)
	}
	got := decode[struct {
		Latest []api.Wear `json:"latest"`
		Log    []api.Wear `json:"log"`
	}](t, rec)
	if len(got.Latest) != 1 || *got.Latest[0].PercentageUsed != 3 ||
		len(got.Log) != 1 || got.Log[0].Reason != "daily" {
		t.Errorf("wear = %+v", got)
	}
	if rec := get(h, "/api/wear?limit=0"); rec.Code != 400 {
		t.Errorf("limit=0 status = %d", rec.Code)
	}
	if rec := get(h, "/api/wear?limit=20000"); rec.Code != 200 {
		t.Errorf("limit=20000 status = %d", rec.Code)
	}
	if rec := get(h, "/api/wear?limit=20001"); rec.Code != 400 {
		t.Errorf("limit=20001 status = %d", rec.Code)
	}
	if rec := get(h, "/api/wear?range=14d"); rec.Code != 400 {
		t.Errorf("range=14d status = %d", rec.Code)
	}
	if rec := get(h, "/api/wear?range=30d"); rec.Code != 200 {
		t.Errorf("range=30d status = %d", rec.Code)
	}
	if mon.wearSince == 0 {
		t.Error("range=30d did not reach the monitor as a lower bound")
	}
}

func TestMetricsExposition(t *testing.T) {
	at := time.Date(2026, 1, 1, 10, 0, 0, 0, time.UTC)
	pct, tbw, hours := int64(3), int64(1000), int64(200)
	mon := &fakeMonitor{overview: api.Overview{
		TS: at,
		Temps: []api.TempReading{
			{Key: `nvme:nvme0`, Chip: "nvme", Label: "Composite", Celsius: 35.5},
			{Key: `k10temp:Tctl`, Chip: "k10temp", Label: `T"ctl`, Celsius: 60},
		},
		Host: &api.HostSnapshot{CPUPct: 25, MemUsed: 512, MemTotal: 1024, MemPct: 50,
			Load1: 0.5, NetRxBps: 100, NetTxBps: 200},
		Containers: []api.ContainerSnapshot{{Name: "web", Image: "nginx", ID: "abc",
			CPUPct: 12.5, MemUsed: 10, MemLimit: 20, MemPct: 50, NetRxBps: 1, NetTxBps: 2}},
		Volumes: []api.VolumeSnapshot{{Name: "/", Used: 40, Total: 100, UsedPct: 40}},
		Smart: []api.DiskSmart{{Disk: "nvme0n1", Model: "Samsung", Serial: "S1",
			PercentageUsed: &pct, TBWBytes: &tbw, PowerOnHours: &hours}},
		Collectors: map[string]api.CollectorStatus{
			"temp":   {LastSuccess: at, DurationMS: 2},
			"docker": {LastError: "socket gone"},
		},
	}}

	_, h := testServer(t, mon)
	rec := get(h, "/metrics")
	if rec.Code != 200 || !strings.HasPrefix(rec.Header().Get("Content-Type"), "text/plain") {
		t.Fatalf("status = %d, content-type = %q", rec.Code, rec.Header().Get("Content-Type"))
	}
	body := rec.Body.String()
	for _, want := range []string{
		`mon_temperature_celsius{chip="k10temp",label="T\"ctl",sensor="k10temp:Tctl"} 60`,
		`mon_temperature_celsius{chip="nvme",label="Composite",sensor="nvme:nvme0"} 35.5`,
		"mon_host_cpu_percent 25",
		"mon_container_cpu_percent{id=\"abc\",image=\"nginx\",name=\"web\"} 12.5",
		"mon_volume_used_percent{volume=\"/\"} 40",
		`mon_smart_percentage_used{disk="nvme0n1",model="Samsung",serial="S1"} 3`,
		"mon_smart_data_written_bytes{disk=\"nvme0n1\",model=\"Samsung\",serial=\"S1\"} 1000",
		`mon_collector_up{collector="temp"} 1`,
		`mon_collector_up{collector="docker"} 0`,
		`mon_collector_last_success_timestamp_seconds{collector="temp"} 1767261600`,
	} {
		if !strings.Contains(body, want) {
			t.Errorf("metrics missing %q\n%s", want, body)
		}
	}
}

func TestHealthAndStatic(t *testing.T) {
	_, h := testServer(t, &fakeMonitor{})
	if rec := get(h, "/healthz"); rec.Code != 200 || !strings.Contains(rec.Body.String(), `"ok":true`) {
		t.Errorf("health = %d %s", rec.Code, rec.Body)
	}
	if rec := get(h, "/"); rec.Code != 200 ||
		!strings.HasPrefix(rec.Header().Get("Content-Type"), "text/html") {
		t.Errorf("index = %d %q", rec.Code, rec.Header().Get("Content-Type"))
	}
	if rec := get(h, "/some/spa/route"); rec.Code != 200 {
		t.Errorf("spa fallback = %d", rec.Code)
	}
	if rec := get(h, "/api/nope"); rec.Code != 404 {
		t.Errorf("unknown api = %d", rec.Code)
	}
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/api/overview", nil))
	if rec.Code != http.StatusMethodNotAllowed {
		t.Errorf("POST overview = %d, want 405", rec.Code)
	}
}
