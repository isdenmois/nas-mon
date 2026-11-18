package collect

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

// Realistic docker stats documents are large: the daemon sends the whole
// cgroup sample, and the client decodes it on every tick. The fixture keeps
// the fields dockerd actually fills so the decode cost is representative.
const fixtureContainers = 25

func statsFixture() []byte {
	memoryStats := map[string]uint64{
		"active_anon": 73400320, "active_file": 41943040, "anon": 75500000,
		"anon_thp": 2097152, "file": 52428800, "file_dirty": 4096,
		"file_mapped": 8388608, "file_writeback": 0, "inactive_anon": 1048576,
		"inactive_file": 31457280, "kernel_stack": 16384, "pgactivate": 123456,
		"pgdeactivate": 2345, "pgfault": 9876543, "pglazyfree": 12,
		"pglazyfreed": 3, "pgmajfault": 4567, "pgrefill": 8901, "pgscan": 12000,
		"pgsteal": 11000, "pgthrottle": 7, "shmem": 65536, "shmem_thp": 0,
		"slab": 1048576, "slab_reclaimable": 786432, "slab_unreclaimable": 262144,
		"sock": 8192, "swap": 0, "unevictable": 0, "workingset_activate": 100,
		"workingset_nodereclaim": 0, "workingset_refault": 55,
		"total_inactive_file": 31457280, "total_active_file": 41943040,
		"total_unevictable": 0, "total_active_anon": 73400320,
		"total_inactive_anon": 1048576, "total_shmem": 65536,
		"total_slab": 1048576, "total_kernel_stack": 16384,
	}
	percpu := make([]uint64, 8)
	for i := range percpu {
		percpu[i] = uint64(1234567890 + i*7654321)
	}
	doc := map[string]any{
		"read": "2026-01-02T03:04:05.123456789Z", "preread": "2026-01-02T03:04:04.123456789Z",
		"pids_stats": map[string]any{"current": 12, "limit": 0},
		"blkio_stats": map[string]any{
			"io_service_bytes_recursive": []map[string]any{
				{"major": 8, "minor": 0, "op": "read", "value": 1234567},
				{"major": 8, "minor": 0, "op": "write", "value": 7654321},
				{"major": 259, "minor": 1, "op": "read", "value": 42},
			},
			"io_serviced_recursive": []map[string]any{
				{"major": 8, "minor": 0, "op": "read", "value": 99},
			},
			"io_queue_recursive":        []any{},
			"io_service_time_recursive": []any{},
			"io_wait_time_recursive":    []any{},
			"io_merged_recursive":       []any{},
			"io_time_recursive":         []any{},
			"sectors_recursive":         []any{},
		},
		"num_procs": 0, "storage_stats": map[string]any{},
		"name": "/worker", "id": "c0ffee1234567890",
		"cpu_stats": map[string]any{
			"cpu_usage": map[string]any{
				"total_usage": 98765432109, "percpu_usage": percpu,
				"usage_in_kernelmode": 123456789, "usage_in_usermode": 987654321,
			},
			"system_cpu_usage": 1234567890123, "online_cpus": 8,
			"throttling_data": map[string]any{
				"periods": 12345, "throttled_periods": 7, "throttled_time": 987654,
			},
		},
		"precpu_stats": map[string]any{
			"cpu_usage": map[string]any{
				"total_usage": 98765432100, "percpu_usage": percpu,
				"usage_in_kernelmode": 123456780, "usage_in_usermode": 987654000,
			},
			"system_cpu_usage": 1234567880000, "online_cpus": 8,
			"throttling_data": map[string]any{"periods": 12344, "throttled_periods": 7, "throttled_time": 987000},
		},
		"memory_stats": map[string]any{
			"usage": 104857600, "max_usage": 157286400, "stats": memoryStats,
			"limit": 8589934592, "commitbytes": 0, "commitpeakbytes": 0, "failcnt": 0,
		},
		"networks": map[string]any{
			"eth0": map[string]any{
				"rx_bytes": 123456789, "rx_packets": 98765, "rx_errors": 0, "rx_dropped": 3,
				"tx_bytes": 987654321, "tx_packets": 54321, "tx_errors": 0, "tx_dropped": 1,
			},
			"eth1": map[string]any{
				"rx_bytes": 4242, "rx_packets": 42, "rx_errors": 0, "rx_dropped": 0,
				"tx_bytes": 2424, "tx_packets": 24, "tx_errors": 0, "tx_dropped": 0,
			},
		},
	}
	raw, err := json.Marshal(doc)
	if err != nil {
		panic(err)
	}
	return raw
}

// fakeDockerDaemon serves the endpoints Docker.Read touches, with one shared
// stats document, the way dockerd does for a 25-container host.
func fakeDockerDaemon(tb testing.TB) *httptest.Server {
	tb.Helper()
	payload := statsFixture()
	containers := make([]map[string]any, 0, fixtureContainers)
	for i := 0; i < fixtureContainers; i++ {
		containers = append(containers, map[string]any{
			"Id":      fmt.Sprintf("id%04d", i),
			"Names":   []string{fmt.Sprintf("/svc-%02d", i)},
			"Image":   "ghcr.io/example/app:v1.2.3",
			"State":   "running",
			"Status":  "Up 3 days (healthy)",
			"Created": 1767225600,
			"Ports":   []map[string]any{{"PrivatePort": 8080, "PublicPort": 8080, "Type": "tcp"}},
			"Labels":  map[string]string{"com.docker.compose.project": "stack"},
		})
	}
	list, err := json.Marshal(containers)
	if err != nil {
		tb.Fatalf("marshal container list: %v", err)
	}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.URL.Path == "/_ping":
			w.Header().Set("API-Version", "1.44")
			_, _ = w.Write([]byte("OK"))
		case r.URL.Path == "/v1.44/containers/json":
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write(list)
		case len(r.URL.Path) > len("/v1.44/containers/") &&
			r.URL.Path[:len("/v1.44/containers/")] == "/v1.44/containers/":
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write(payload)
		default:
			http.NotFound(w, r)
		}
	}))
	tb.Cleanup(srv.Close)
	return srv
}

// TestReadUsesTheFixturePayload keeps the benchmark honest: the fixture has to
// travel through the real client and produce the values the collector reports.
func TestReadUsesTheFixturePayload(t *testing.T) {
	// Arrange
	srv := fakeDockerDaemon(t)
	d := NewDocker(newDockerClientForTest(srv.URL, srv.Client()), nil)
	now := time.Date(2026, 1, 2, 3, 4, 5, 0, time.UTC)

	// Act
	snaps, err := d.Read(context.Background(), now)

	// Assert
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	if len(snaps) != fixtureContainers {
		t.Fatalf("snapshots = %d, want %d", len(snaps), fixtureContainers)
	}
	first := snaps[0]
	if first.Name != "svc-00" || first.CPUPct <= 0 || first.MemUsed == 0 || first.MemPct <= 0 {
		t.Fatalf("snapshot = %+v", first)
	}
	if first.NetRxBps != 0 || first.NetTxBps != 0 {
		t.Errorf("first tick must report zero rates, got rx=%v tx=%v", first.NetRxBps, first.NetTxBps)
	}
}

// BenchmarkRead25Containers is one collection tick against a 25-container
// host. The tick count per minute is what MON_DOCKER_INTERVAL controls.
func BenchmarkRead25Containers(b *testing.B) {
	srv := fakeDockerDaemon(b)
	d := NewDocker(newDockerClientForTest(srv.URL, srv.Client()), nil)
	payload := len(statsFixture())
	now := time.Date(2026, 1, 2, 3, 4, 5, 0, time.UTC)
	ctx := context.Background()

	b.ReportAllocs()
	b.SetBytes(int64(payload * fixtureContainers))
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if _, err := d.Read(ctx, now.Add(time.Duration(i)*time.Minute)); err != nil {
			b.Fatalf("read: %v", err)
		}
	}
}
