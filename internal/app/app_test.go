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
	const tbw = int64(28_000_000_000_000)
	disk := collect.DiskSmart{
		Disk: "nvme0n1", Model: "Samsung", Serial: "S1",
		PercentageUsed: i64(3), TBWBytes: i64(tbw), PowerOnHours: i64(100),
		MediaErrors: i64(0), Reallocated: i64(0), TempC: i64(40),
	}
	prev := &store.Wear{
		Disk: "nvme0n1", TS: time.Date(2026, 3, 4, 12, 0, 0, 0, time.UTC).Unix(),
		Model: "Samsung", Serial: "S1",
		PercentageUsed: i64(3), TBWBytes: i64(tbw), PowerOnHours: i64(100),
		MediaErrors: i64(0), Reallocated: i64(0), TempC: i64(40),
	}

	cases := []struct {
		name   string
		prev   *store.Wear
		mutate func(*collect.DiskSmart)
		want   bool
		reason string
	}{
		{"first ever", nil, func(*collect.DiskSmart) {}, true, "first"},
		{"idle", prev, func(*collect.DiskSmart) {}, false, ""},
		{"temperature only", prev, func(d *collect.DiskSmart) { d.TempC = i64(55) }, false, ""},
		{"power-on only", prev, func(d *collect.DiskSmart) { d.PowerOnHours = i64(101) }, false, ""},
		{"wear percentage up", prev, func(d *collect.DiskSmart) { d.PercentageUsed = i64(4) }, true, "wear"},
		{"tbw below step", prev, func(d *collect.DiskSmart) { d.TBWBytes = i64(tbw + 50_000_000_000) }, false, ""},
		{"tbw at step", prev, func(d *collect.DiskSmart) { d.TBWBytes = i64(tbw + wearLogTBWStep) }, true, "tbw"},
		{"tbw above step", prev, func(d *collect.DiskSmart) { d.TBWBytes = i64(tbw + 2*wearLogTBWStep) }, true, "tbw"},
		{"tbw reset", prev, func(d *collect.DiskSmart) { d.TBWBytes = i64(tbw - 1) }, true, "tbw-reset"},
		{"tbw missing now", prev, func(d *collect.DiskSmart) { d.TBWBytes = nil }, false, ""},
		{"errors appear as zero", prev, func(d *collect.DiskSmart) { d.MediaErrors = nil; d.Reallocated = nil }, false, ""},
		{"wear appears as zero", prev, func(d *collect.DiskSmart) { d.PercentageUsed = nil }, false, ""},
		{"media errors", prev, func(d *collect.DiskSmart) { d.MediaErrors = i64(1) }, true, "errors"},
		{"reallocated", prev, func(d *collect.DiskSmart) { d.Reallocated = i64(1) }, true, "errors"},
		{"errors with same count", prev, func(d *collect.DiskSmart) { d.MediaErrors = i64(0) }, false, ""},
		{"serial changed", prev, func(d *collect.DiskSmart) { d.Serial = "S2" }, true, "identity"},
		{"model changed", prev, func(d *collect.DiskSmart) { d.Model = "WD" }, true, "identity"},
		{"wear and tbw", prev, func(d *collect.DiskSmart) {
			d.PercentageUsed = i64(4)
			d.TBWBytes = i64(tbw + wearLogTBWStep)
		}, true, "wear+tbw"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			d := disk
			tc.mutate(&d)
			got, reason := shouldAppendWear(tc.prev, d, wearLogTBWStep)
			if got != tc.want || reason != tc.reason {
				t.Errorf("got %v %q, want %v %q", got, reason, tc.want, tc.reason)
			}
		})
	}
}

func TestShouldAppendWearErrorsAppearNonZero(t *testing.T) {
	prev := &store.Wear{Disk: "sda", Model: "Samsung"}
	d := collect.DiskSmart{Disk: "sda", Model: "Samsung", MediaErrors: i64(2)}
	got, reason := shouldAppendWear(prev, d, wearLogTBWStep)
	if !got || reason != "errors" {
		t.Errorf("got %v %q, want true errors", got, reason)
	}
}

func TestShouldAppendWearFirstTBWValue(t *testing.T) {
	prev := &store.Wear{Disk: "sda", Model: "Samsung", TBWBytes: nil}
	d := collect.DiskSmart{Disk: "sda", Model: "Samsung", TBWBytes: i64(100)}
	got, reason := shouldAppendWear(prev, d, wearLogTBWStep)
	if !got || reason != "tbw" {
		t.Errorf("got %v %q, want true tbw", got, reason)
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
