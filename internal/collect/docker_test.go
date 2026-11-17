package collect

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"regexp"
	"strings"
	"sync"
	"testing"
	"time"
)

func stats(usage, cache, limit uint64, rx, tx uint64) dockerStats {
	var st dockerStats
	st.MemoryStats.Usage = usage
	st.MemoryStats.Limit = limit
	st.MemoryStats.Stats = map[string]uint64{"inactive_file": cache}
	st.Networks = map[string]struct {
		RxBytes uint64 `json:"rx_bytes"`
		TxBytes uint64 `json:"tx_bytes"`
	}{"eth0": {RxBytes: rx, TxBytes: tx}}
	return st
}

func TestCPUPercent(t *testing.T) {
	var st dockerStats
	st.PreCPUStats.SystemCPUUsage = 1000
	st.PreCPUStats.CPUUsage.TotalUsage = 100
	st.CPUStats.SystemCPUUsage = 2000
	st.CPUStats.CPUUsage.TotalUsage = 300
	st.CPUStats.OnlineCPUs = 4
	if got := cpuPercent(st); got != 80 {
		t.Errorf("cpu = %v, want 80", got)
	}

	st.CPUStats.SystemCPUUsage = 500 // counter reset
	if got := cpuPercent(st); got != 0 {
		t.Errorf("reset cpu = %v, want 0", got)
	}
}

func TestMemUsedSubtractsCache(t *testing.T) {
	st := stats(1000, 400, 8000, 0, 0)
	if got := memUsed(st); got != 600 {
		t.Errorf("used = %d, want 600", got)
	}
	st.MemoryStats.Stats = map[string]uint64{"cache": 200}
	if got := memUsed(st); got != 800 {
		t.Errorf("cgroup v1 used = %d, want 800", got)
	}
	st.MemoryStats.Stats = map[string]uint64{"inactive_file": 2000}
	if got := memUsed(st); got != 0 {
		t.Errorf("cache above usage = %d, want 0", got)
	}
}

type fakeDockerAPI struct {
	mu         sync.Mutex
	containers []containerSummary
	stats      map[string]dockerStats
	fail       map[string]error
	calls      []string
}

func (f *fakeDockerAPI) Containers(context.Context) ([]containerSummary, error) {
	return f.containers, nil
}

func (f *fakeDockerAPI) Stats(_ context.Context, id string) (dockerStats, error) {
	f.mu.Lock()
	f.calls = append(f.calls, id)
	f.mu.Unlock()
	if err := f.fail[id]; err != nil {
		return dockerStats{}, err
	}
	return f.stats[id], nil
}

func TestDockerReadRatesExclusionsAndErrors(t *testing.T) {
	api := &fakeDockerAPI{
		containers: []containerSummary{
			{ID: "aaa", Names: []string{"/web"}, Image: "nginx", State: "running"},
			{ID: "bbb", Names: []string{"/db"}, Image: "postgres", State: "running"},
			{ID: "ccc", Names: []string{"/ignore-me"}, State: "running"},
			{ID: "ddd", Names: []string{"/broken"}, State: "running"},
		},
		stats: map[string]dockerStats{
			"aaa": stats(1000, 400, 2000, 1000, 2000),
			"bbb": stats(500, 0, 0, 0, 0),
		},
		fail: map[string]error{"ddd": errors.New("container vanished")},
	}
	d := NewDocker(api, regexp.MustCompile(`^ignore-`))
	base := time.Unix(1000, 0)

	first, err := d.Read(context.Background(), base)
	if err == nil || !strings.Contains(err.Error(), "broken") {
		t.Errorf("err = %v, want broken container error", err)
	}
	if len(first) != 2 {
		t.Fatalf("containers = %+v, want web and db", first)
	}
	web := first[0]
	if web.Name != "db" || first[1].Name != "web" {
		t.Errorf("not sorted by name: %+v", first)
	}
	web = first[1]
	if web.CPUPct != 0 || web.NetRxBps != 0 || web.NetTxBps != 0 {
		t.Errorf("first poll should have zero rates: %+v", web)
	}
	if web.MemUsed != 600 || web.MemPct != 30 {
		t.Errorf("web memory = %+v", web)
	}

	api.stats["aaa"] = stats(1000, 400, 2000, 7000, 4000)
	second, _ := d.Read(context.Background(), base.Add(10*time.Second))
	web = second[1]
	if web.NetRxBps != 600 || web.NetTxBps != 200 {
		t.Errorf("rates = %v/%v, want 600/200", web.NetRxBps, web.NetTxBps)
	}
	if web.MemPct != 30 {
		t.Errorf("mem pct = %v", web.MemPct)
	}

	if _, ok := d.prev["ccc"]; ok {
		t.Error("excluded container kept in rate state")
	}
}

func TestDockerClientTalksToVersionedAPI(t *testing.T) {
	var sawStats bool
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.URL.Path == "/_ping":
			w.Header().Set("API-Version", "1.44")
			w.Write([]byte("OK"))
		case r.URL.Path == "/v1.44/containers/json":
			w.Header().Set("Content-Type", "application/json")
			w.Write([]byte(`[{"Id":"abc","Names":["/web"],"Image":"nginx","State":"running"}]`))
		case r.URL.Path == "/v1.44/containers/abc/stats":
			sawStats = true
			if r.URL.Query().Get("stream") != "false" {
				t.Errorf("stream query = %q", r.URL.Query().Get("stream"))
			}
			w.Header().Set("Content-Type", "application/json")
			w.Write([]byte(`{
				"cpu_stats":{"cpu_usage":{"total_usage":300},"system_cpu_usage":2000,"online_cpus":2},
				"precpu_stats":{"cpu_usage":{"total_usage":100},"system_cpu_usage":1000},
				"memory_stats":{"usage":1000,"limit":4000,"stats":{"inactive_file":400}},
				"networks":{"eth0":{"rx_bytes":10,"tx_bytes":20}}
			}`))
		default:
			t.Errorf("unexpected request %s", r.URL.String())
			http.NotFound(w, r)
		}
	}))
	defer srv.Close()

	c := newDockerClientForTest(srv.URL, srv.Client())
	cs, err := c.Containers(context.Background())
	if err != nil || len(cs) != 1 || containerName(cs[0]) != "web" {
		t.Fatalf("containers = %+v, %v", cs, err)
	}
	st, err := c.Stats(context.Background(), "abc")
	if err != nil || !sawStats {
		t.Fatalf("stats err = %v", err)
	}
	if cpuPercent(st) != 40 {
		t.Errorf("cpu = %v, want 40", cpuPercent(st))
	}
	if memUsed(st) != 600 {
		t.Errorf("mem = %d", memUsed(st))
	}
}

func TestDockerClientReportsHTTPError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/_ping" {
			w.WriteHeader(http.StatusOK)
			return
		}
		http.Error(w, "no such container", http.StatusNotFound)
	}))
	defer srv.Close()
	c := newDockerClientForTest(srv.URL, srv.Client())
	if _, err := c.Containers(context.Background()); err == nil ||
		!strings.Contains(err.Error(), "404") {
		t.Fatalf("err = %v", err)
	}
}
