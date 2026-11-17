package app

import (
	"io"
	"log/slog"
	"path/filepath"
	"testing"
	"time"

	"mon/internal/collect"
	"mon/internal/model"
	"mon/internal/recorder"
	"mon/internal/store"
)

func i64(v int64) *int64 { return &v }

func TestShouldAppendWear(t *testing.T) {
	now := time.Date(2026, 3, 4, 12, 0, 0, 0, time.UTC)
	disk := collect.DiskSmart{
		Disk: "nvme0n1", Model: "Samsung", Serial: "S1",
		PercentageUsed: i64(3), TBWBytes: i64(1000), PowerOnHours: i64(100),
		MediaErrors: i64(0), Reallocated: i64(0), TempC: i64(40),
	}
	prev := &store.Wear{
		Disk: "nvme0n1", TS: now.Add(-time.Hour).Unix(), Model: "Samsung", Serial: "S1",
		PercentageUsed: i64(3), TBWBytes: i64(1000), PowerOnHours: i64(100),
		MediaErrors: i64(0), Reallocated: i64(0), TempC: i64(40),
	}

	cases := []struct {
		name   string
		prev   *store.Wear
		mutate func(*collect.DiskSmart)
		now    time.Time
		want   bool
		reason string
	}{
		{"first ever", nil, func(*collect.DiskSmart) {}, now, true, "first"},
		{"idle same day", prev, func(*collect.DiskSmart) {}, now, false, ""},
		{"temperature only", prev, func(d *collect.DiskSmart) { d.TempC = i64(55) }, now, false, ""},
		{"wear changed", prev, func(d *collect.DiskSmart) { d.PercentageUsed = i64(4) }, now, true, "change"},
		{"tbw changed", prev, func(d *collect.DiskSmart) { d.TBWBytes = i64(2000) }, now, true, "change"},
		{"power on hours", prev, func(d *collect.DiskSmart) { d.PowerOnHours = i64(101) }, now, true, "change"},
		{"media errors", prev, func(d *collect.DiskSmart) { d.MediaErrors = i64(2) }, now, true, "change"},
		{"reallocated", prev, func(d *collect.DiskSmart) { d.Reallocated = i64(1) }, now, true, "change"},
		{"identity", prev, func(d *collect.DiskSmart) { d.Serial = "S2" }, now, true, "change"},
		{"nil vs zero", prev, func(d *collect.DiskSmart) { d.MediaErrors = nil }, now, true, "change"},
		{"new day", prev, func(*collect.DiskSmart) {}, now.Add(24 * time.Hour), true, "daily"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			d := disk
			tc.mutate(&d)
			got, reason := shouldAppendWear(tc.prev, d, tc.now)
			if got != tc.want || reason != tc.reason {
				t.Errorf("got %v %q, want %v %q", got, reason, tc.want, tc.reason)
			}
		})
	}
}

func TestFlushWritesClosedMinutesOnlyOnce(t *testing.T) {
	db, err := store.Open(filepath.Join(t.TempDir(), "mon.db"), 0, 0)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	a := &App{db: db, rec: recorder.New(), log: slog.New(slog.NewTextHandler(io.Discard, nil))}

	now := time.Date(2026, 3, 4, 12, 0, 30, 0, time.UTC)
	a.rec.AddAt(model.KindTemp, "cpu", 40, now.Add(-2*time.Minute))
	a.rec.AddAt(model.KindTemp, "cpu", 50, now.Add(-2*time.Minute+10*time.Second))
	a.rec.AddAt(model.KindTemp, "cpu", 99, now) // current minute, must stay buffered

	if err := a.flush(now); err != nil {
		t.Fatal(err)
	}
	from := now.Add(-time.Hour).Unix()
	got, err := db.History(model.KindTemp, []string{"cpu"}, from, now.Unix()+60, store.StepMinute)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 || len(got[0].Points) != 1 {
		t.Fatalf("history = %+v", got)
	}
	if p := got[0].Points[0]; p.Avg != 45 || p.N != 2 {
		t.Errorf("point = %+v, want avg 45 n 2", p)
	}
	if err := a.flush(now); err != nil {
		t.Fatal(err)
	}
	got, _ = db.History(model.KindTemp, []string{"cpu"}, from, now.Unix()+60, store.StepMinute)
	if len(got[0].Points) != 1 {
		t.Errorf("second flush duplicated rows: %+v", got)
	}
}
