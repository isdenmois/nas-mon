package store

import (
	"path/filepath"
	"testing"
	"time"

	"mon/internal/api"
	"mon/internal/domain"
)

func openTest(t *testing.T, retentionMinute, retentionHour time.Duration) *DB {
	t.Helper()
	db, err := Open(filepath.Join(t.TempDir(), "test.db"), retentionMinute, retentionHour)
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	t.Cleanup(func() { db.Close() })
	return db
}

func TestRollupMinuteHourDay(t *testing.T) {
	db := openTest(t, 0, 0)
	base := time.Date(2026, 1, 1, 10, 0, 0, 0, time.UTC).Unix()

	if err := db.InsertMinute([]domain.Point{
		{Kind: domain.Temp, Name: "cpu:Tctl", TS: base, Avg: 40, Min: 30, Max: 50, N: 2},
		{Kind: domain.Temp, Name: "cpu:Tctl", TS: base + 60, Avg: 50, Min: 45, Max: 60, N: 3},
		{Kind: domain.Temp, Name: "cpu:Tctl", TS: base + 3600, Avg: 60, Min: 55, Max: 65, N: 4},
	}); err != nil {
		t.Fatalf("insert: %v", err)
	}

	hourly := history(t, db, domain.Hour)
	assertPoints(t, hourly, 2)
	assertPoint(t, hourly[0].Points[0], base, 46, 30, 60, 5)
	assertPoint(t, hourly[0].Points[1], base+3600, 60, 55, 65, 4)

	daily := history(t, db, domain.Day)
	assertPoints(t, daily, 1)
	// (46*5 + 60*4) / 9 = 52.222...
	if got := daily[0].Points[0].Avg; got < 52.22 || got > 52.23 {
		t.Errorf("daily avg = %v, want ~52.222", got)
	}
	if daily[0].Points[0].N != 9 {
		t.Errorf("daily n = %d, want 9", daily[0].Points[0].N)
	}

	// A new minute in the first hour must recompute that hour from all minutes.
	if err := db.InsertMinute([]domain.Point{
		{Kind: domain.Temp, Name: "cpu:Tctl", TS: base + 120, Avg: 70, Min: 70, Max: 70, N: 1},
	}); err != nil {
		t.Fatalf("insert second: %v", err)
	}
	hourly = history(t, db, domain.Hour)
	assertPoint(t, hourly[0].Points[0], base, 50, 30, 70, 6)
}

func TestMinuteAccumulatesOnConflict(t *testing.T) {
	db := openTest(t, 0, 0)
	base := time.Date(2026, 1, 1, 10, 0, 0, 0, time.UTC).Unix()
	p := domain.Point{Kind: domain.Temp, Name: "x", TS: base, Avg: 40, Min: 40, Max: 40, N: 1}
	if err := db.InsertMinute([]domain.Point{p}); err != nil {
		t.Fatal(err)
	}
	p = domain.Point{Kind: domain.Temp, Name: "x", TS: base, Avg: 50, Min: 50, Max: 50, N: 1}
	if err := db.InsertMinute([]domain.Point{p}); err != nil {
		t.Fatal(err)
	}
	m := history(t, db, domain.Minute)
	assertPoint(t, m[0].Points[0], base, 45, 40, 50, 2)
}

func TestPruneKeepsDailyForever(t *testing.T) {
	db := openTest(t, 48*time.Hour, 90*24*time.Hour)
	now := time.Now().UTC().Truncate(time.Minute)
	old := now.Add(-100 * 24 * time.Hour).Unix()

	if err := db.InsertMinute([]domain.Point{
		{Kind: domain.HostCPU, Name: "host", TS: old, Avg: 10, Min: 5, Max: 15, N: 1},
	}); err != nil {
		t.Fatal(err)
	}
	if err := db.Prune(now); err != nil {
		t.Fatal(err)
	}
	if got := historyKind(t, db, domain.HostCPU, domain.Minute); len(got) != 0 {
		t.Errorf("minute rows survived prune: %d", len(got))
	}
	if got := historyKind(t, db, domain.HostCPU, domain.Hour); len(got) != 0 {
		t.Errorf("hour rows survived prune: %d", len(got))
	}
	if got := historyKind(t, db, domain.HostCPU, domain.Day); len(got) != 1 {
		t.Errorf("daily rows = %d, want 1", len(got))
	}
}

func TestHistoryFiltersByNameAndStep(t *testing.T) {
	db := openTest(t, 0, 0)
	base := time.Date(2026, 1, 1, 10, 0, 0, 0, time.UTC).Unix()
	if err := db.InsertMinute([]domain.Point{
		{Kind: domain.Temp, Name: "a", TS: base, Avg: 1, Min: 1, Max: 1, N: 1},
		{Kind: domain.Temp, Name: "b", TS: base, Avg: 2, Min: 2, Max: 2, N: 1},
		{Kind: domain.HostCPU, Name: "host", TS: base, Avg: 3, Min: 3, Max: 3, N: 1},
	}); err != nil {
		t.Fatal(err)
	}
	got, err := db.History(domain.Temp, []string{"b"}, base, base+60, domain.Minute)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 || got[0].Name != "b" {
		t.Fatalf("filtered history = %+v", got)
	}
	all := history(t, db, domain.Minute)
	if len(all) != 2 {
		t.Errorf("all temp minute series = %d, want 2", len(all))
	}
	if _, err := db.History(domain.Temp, nil, base, base+60, domain.Resolution(0)); err == nil {
		t.Error("unknown step accepted")
	}
}

func TestWearLogRoundTrip(t *testing.T) {
	db := openTest(t, 0, 0)
	pct, tbw := int64(3), int64(123456789)
	w := Wear{Disk: "nvme0n1", TS: 100, Model: "Samsung", Serial: "S1",
		PercentageUsed: &pct, TBWBytes: &tbw, Reason: "change"}
	if err := db.AppendWear(w); err != nil {
		t.Fatal(err)
	}
	if err := db.AppendWear(Wear{Disk: "nvme1n1", TS: 200, Model: "WD"}); err != nil {
		t.Fatal(err)
	}

	last, err := db.LastWear("nvme0n1")
	if err != nil || last == nil {
		t.Fatalf("last = %+v, %v", last, err)
	}
	if *last.PercentageUsed != 3 || *last.TBWBytes != 123456789 || last.Model != "Samsung" {
		t.Errorf("last = %+v", last)
	}
	if last, _ := db.LastWear("missing"); last != nil {
		t.Errorf("missing disk = %+v", last)
	}

	latest, err := db.WearLatest()
	if err != nil || len(latest) != 2 {
		t.Fatalf("latest = %+v, %v", latest, err)
	}
	log, err := db.WearLog(10)
	if err != nil || len(log) != 2 || log[0].Disk != "nvme1n1" {
		t.Errorf("log = %+v, %v", log, err)
	}
}

func history(t *testing.T, db *DB, step domain.Resolution) []api.SeriesHistory {
	return historyKind(t, db, domain.Temp, step)
}

func historyKind(t *testing.T, db *DB, kind domain.Kind, step domain.Resolution) []api.SeriesHistory {
	t.Helper()
	from := time.Date(2020, 1, 1, 0, 0, 0, 0, time.UTC).Unix()
	to := time.Date(2030, 1, 1, 0, 0, 0, 0, time.UTC).Unix()
	got, err := db.History(kind, nil, from, to, step)
	if err != nil {
		t.Fatalf("history %s: %v", step, err)
	}
	return got
}

func assertPoints(t *testing.T, h []api.SeriesHistory, want int) {
	t.Helper()
	if len(h) != 1 || len(h[0].Points) != want {
		t.Fatalf("history = %+v, want 1 series with %d points", h, want)
	}
}

func assertPoint(t *testing.T, p api.SeriesPoint, ts int64, avg, minV, maxV float64, n int64) {
	t.Helper()
	if p.TS != ts || p.Avg != avg || p.Min != minV || p.Max != maxV || p.N != n {
		t.Errorf("point = %+v, want ts=%d avg=%v min=%v max=%v n=%d",
			p, ts, avg, minV, maxV, n)
	}
}
