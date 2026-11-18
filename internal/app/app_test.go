package app

import (
	"context"
	"io"
	"log/slog"
	"path/filepath"
	"testing"
	"time"

	"mon/internal/collect"
	"mon/internal/config"
	"mon/internal/domain"
	"mon/internal/store"
)

func testConfig(t *testing.T, aggregate time.Duration) config.Config {
	t.Helper()
	return config.Config{
		Port:              0, // let the kernel pick a free port
		DBPath:            filepath.Join(t.TempDir(), "mon.db"),
		DockerSocket:      filepath.Join(t.TempDir(), "docker.sock"),
		VolumesDir:        t.TempDir(),
		AggregateInterval: aggregate,
		TempWarnC:         80,
		LogLevel:          "error",
	}
}

func discardLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(io.Discard, nil))
}

// A stop must not cost a whole aggregate interval of closed buckets: the last
// flush runs when the loop is cancelled.
func TestShutdownFlushesClosedBuckets(t *testing.T) {
	cfg := testConfig(t, time.Hour) // too long to fire on its own
	a, err := New(cfg, discardLogger())
	if err != nil {
		t.Fatalf("new: %v", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() { done <- a.Run(ctx) }()

	closed := time.Now().Add(-2 * time.Minute)
	a.mon.IngestTemps(closed, []collect.TempReading{{Key: "cpu:Tctl", Celsius: 42}})

	cancel()
	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("run: %v", err)
		}
	case <-time.After(10 * time.Second):
		t.Fatal("run did not stop")
	}

	db, err := store.Open(cfg.DBPath, store.Retention{})
	if err != nil {
		t.Fatalf("reopen: %v", err)
	}
	defer db.Close()
	series, err := db.ListSeries()
	if err != nil {
		t.Fatalf("list series: %v", err)
	}
	if len(series) != 1 || series[0].Kind != domain.Temp {
		t.Fatalf("series after shutdown flush = %+v", series)
	}
	got, err := db.History([]int64{series[0].ID}, 0, time.Now().Unix(), domain.Minute)
	if err != nil {
		t.Fatalf("history: %v", err)
	}
	if len(got) != 1 || len(got[0].Points) != 1 {
		t.Fatalf("history after shutdown flush = %+v", got)
	}
	if p := got[0].Points[0]; p.Avg != 42 {
		t.Errorf("point = %+v, want one sample of 42", p)
	}
}
