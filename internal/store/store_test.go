package store

import (
	"path/filepath"
	"strings"
	"testing"
	"time"

	"mon/internal/api"
	"mon/internal/domain"
)

func openTest(t *testing.T, ret Retention) *DB {
	t.Helper()
	db, err := Open(filepath.Join(t.TempDir(), "test.db"), ret)
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	t.Cleanup(func() { db.Close() })
	return db
}

func TestRollupMinuteHourDay(t *testing.T) {
	db := openTest(t, Retention{})
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
	assertPoint(t, hourly[0].Points[0], base, 46)
	assertStored(t, db, domain.Hour, base, 46, 30, 60, 5)
	assertPoint(t, hourly[0].Points[1], base+3600, 60)
	assertStored(t, db, domain.Hour, base+3600, 60, 55, 65, 4)

	daily := history(t, db, domain.Day)
	assertPoints(t, daily, 1)
	// (46*5 + 60*4) / 9 = 52.222...
	if got := daily[0].Points[0].Avg; got < 52.22 || got > 52.23 {
		t.Errorf("daily avg = %v, want ~52.222", got)
	}
	assertStored(t, db, domain.Day, domain.Day.Bucket(base), daily[0].Points[0].Avg, 30, 65, 9)

	// A new minute in the first hour must recompute that hour from all minutes.
	if err := db.InsertMinute([]domain.Point{
		{Kind: domain.Temp, Name: "cpu:Tctl", TS: base + 120, Avg: 70, Min: 70, Max: 70, N: 1},
	}); err != nil {
		t.Fatalf("insert second: %v", err)
	}
	hourly = history(t, db, domain.Hour)
	assertPoint(t, hourly[0].Points[0], base, 50)
	assertStored(t, db, domain.Hour, base, 50, 30, 70, 6)
}

func TestMinuteAccumulatesOnConflict(t *testing.T) {
	db := openTest(t, Retention{})
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
	assertPoint(t, m[0].Points[0], base, 45)
	assertStored(t, db, domain.Minute, base, 45, 40, 50, 2)
}

func TestPruneRespectsEveryResolution(t *testing.T) {
	db := openTest(t, Retention{Minute: 48 * time.Hour, Hour: 90 * 24 * time.Hour, Day: 5 * 365 * 24 * time.Hour})
	now := time.Now().UTC().Truncate(time.Minute)
	recent := now.Add(-100 * 24 * time.Hour).Unix()
	ancient := now.Add(-6 * 365 * 24 * time.Hour).Unix()

	for _, ts := range []int64{recent, ancient} {
		if err := db.InsertMinute([]domain.Point{
			{Kind: domain.HostCPU, Name: "host", TS: ts, Avg: 10, Min: 5, Max: 15, N: 1},
		}); err != nil {
			t.Fatal(err)
		}
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
	days := historyKind(t, db, domain.HostCPU, domain.Day)
	if len(days) != 1 || len(days[0].Points) != 1 {
		t.Fatalf("daily rows = %+v, want only the 100-day-old one", days)
	}
	if got := days[0].Points[0].TS; got != domain.Day.Bucket(recent) {
		t.Errorf("surviving day = %d, want %d", got, domain.Day.Bucket(recent))
	}
}

func TestHistoryFiltersByID(t *testing.T) {
	db := openTest(t, Retention{})
	base := time.Date(2026, 1, 1, 10, 0, 0, 0, time.UTC).Unix()
	if err := db.InsertMinute([]domain.Point{
		{Kind: domain.Temp, Name: "a", TS: base, Avg: 1, Min: 1, Max: 1, N: 1},
		{Kind: domain.Temp, Name: "b", TS: base, Avg: 2, Min: 2, Max: 2, N: 1},
		{Kind: domain.HostCPU, Name: "host", TS: base, Avg: 3, Min: 3, Max: 3, N: 1},
	}); err != nil {
		t.Fatal(err)
	}
	all := history(t, db, domain.Minute)
	if len(all) != 2 {
		t.Fatalf("temp minute series = %d, want 2", len(all))
	}
	var idB int64
	for _, s := range all {
		if s.Name == "b" {
			idB = s.ID
		}
	}
	got, err := db.History([]int64{idB}, base, base+60, domain.Minute)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 || got[0].Name != "b" {
		t.Fatalf("filtered history = %+v", got)
	}
	if _, err := db.History([]int64{idB}, base, base+60, domain.Resolution(0)); err == nil {
		t.Error("unknown resolution accepted")
	}
}

func TestWearLogRoundTrip(t *testing.T) {
	db := openTest(t, Retention{})
	pct, tbw := int64(3), int64(123456789)
	w := domain.WearObservation{Disk: "nvme0n1", TS: 100, Model: "Samsung", Serial: "S1",
		PercentageUsed: &pct, TBWBytes: &tbw,
		Reasons: []domain.WearReason{domain.ReasonWear, domain.ReasonTBW}}
	if err := db.AppendWear(w); err != nil {
		t.Fatal(err)
	}
	if err := db.AppendWear(domain.WearObservation{Disk: "nvme1n1", TS: 200, Model: "WD"}); err != nil {
		t.Fatal(err)
	}

	last, err := db.LastWear("nvme0n1")
	if err != nil || last == nil {
		t.Fatalf("last = %+v, %v", last, err)
	}
	if *last.PercentageUsed != 3 || *last.TBWBytes != 123456789 || last.Model != "Samsung" {
		t.Errorf("last = %+v", last)
	}
	if got := domain.FormatReasons(last.Reasons); got != "wear+tbw" {
		t.Errorf("reasons = %q, want wear+tbw", got)
	}
	if last, _ := db.LastWear("missing"); last != nil {
		t.Errorf("missing disk = %+v", last)
	}

	latest, err := db.WearLatest()
	if err != nil || len(latest) != 2 {
		t.Fatalf("latest = %+v, %v", latest, err)
	}
	log, err := db.WearLog(10, 0)
	if err != nil || len(log) != 2 || log[0].Disk != "nvme1n1" {
		t.Errorf("log = %+v, %v", log, err)
	}
}

func seriesIDs(t *testing.T, db *DB, kind domain.Kind) []int64 {
	t.Helper()
	series, err := db.ListSeries()
	if err != nil {
		t.Fatalf("list series: %v", err)
	}
	var ids []int64
	for _, s := range series {
		if s.Kind == kind {
			ids = append(ids, s.ID)
		}
	}
	return ids
}

func history(t *testing.T, db *DB, step domain.Resolution) []api.SeriesHistory {
	return historyKind(t, db, domain.Temp, step)
}

func historyKind(t *testing.T, db *DB, kind domain.Kind, step domain.Resolution) []api.SeriesHistory {
	t.Helper()
	ids := seriesIDs(t, db, kind)
	from := time.Date(2020, 1, 1, 0, 0, 0, 0, time.UTC).Unix()
	to := time.Date(2030, 1, 1, 0, 0, 0, 0, time.UTC).Unix()
	got, err := db.History(ids, from, to, step)
	if err != nil {
		t.Fatalf("history %v: %v", step, err)
	}
	return got
}

func assertPoints(t *testing.T, h []api.SeriesHistory, want int) {
	t.Helper()
	if len(h) != 1 || len(h[0].Points) != want {
		t.Fatalf("history = %+v, want 1 series with %d points", h, want)
	}
}

// assertPoint checks what the API returns: the bucket timestamp and average.
func assertPoint(t *testing.T, p api.SeriesPoint, ts int64, avg float64) {
	t.Helper()
	if p.TS != ts || p.Avg != avg {
		t.Errorf("point = %+v, want ts=%d avg=%v", p, ts, avg)
	}
}

// assertStored checks the aggregation columns the rollup produced. They stay in
// the database even though the history API only serves ts and avg.
func assertStored(t *testing.T, db *DB, res domain.Resolution, ts int64, avg, minV, maxV float64, n int64) {
	t.Helper()
	var gotAvg, gotMin, gotMax float64
	var gotN int64
	err := db.sql.QueryRow(
		`SELECT avg, min, max, n FROM `+res.Table()+` WHERE ts = ?`, ts).
		Scan(&gotAvg, &gotMin, &gotMax, &gotN)
	if err != nil {
		t.Fatalf("read %s row at %d: %v", res, ts, err)
	}
	if gotAvg != avg || gotMin != minV || gotMax != maxV || gotN != n {
		t.Errorf("%s row = avg %v min %v max %v n %d, want avg %v min %v max %v n %d",
			res, gotAvg, gotMin, gotMax, gotN, avg, minV, maxV, n)
	}
}

func TestOpenIsIdempotent(t *testing.T) {
	path := filepath.Join(t.TempDir(), "reopen.db")
	first, err := Open(path, Retention{})
	if err != nil {
		t.Fatalf("first open: %v", err)
	}
	base := time.Date(2026, 1, 1, 10, 0, 0, 0, time.UTC).Unix()
	if err := first.InsertMinute([]domain.Point{
		{Kind: domain.Temp, Name: "cpu", TS: base, Avg: 40, Min: 40, Max: 40, N: 1},
	}); err != nil {
		t.Fatalf("insert: %v", err)
	}
	if err := first.Close(); err != nil {
		t.Fatal(err)
	}

	second, err := Open(path, Retention{})
	if err != nil {
		t.Fatalf("second open: %v", err)
	}
	defer second.Close()
	got, err := second.History(seriesIDs(t, second, domain.Temp), base, base+60, domain.Minute)
	if err != nil {
		t.Fatalf("history after reopen: %v", err)
	}
	if len(got) != 1 || len(got[0].Points) != 1 || got[0].Points[0].Avg != 40 {
		t.Errorf("history after reopen = %+v", got)
	}
}

func TestWearAppendIsIdempotent(t *testing.T) {
	db := openTest(t, Retention{})
	row := domain.WearObservation{Disk: "nvme0n1", TS: 100, Model: "Samsung",
		Reasons: []domain.WearReason{domain.ReasonFirst}}
	for i := 0; i < 2; i++ {
		if err := db.AppendWear(row); err != nil {
			t.Fatal(err)
		}
	}
	log, err := db.WearLog(10, 0)
	if err != nil {
		t.Fatal(err)
	}
	if len(log) != 1 {
		t.Errorf("retried append duplicated rows: %+v", log)
	}
}

// A delayed or long-lived flush must not re-aggregate every hour between the
// oldest and newest point: only buckets that actually received data change.
func TestRollupCoversOnlyTouchedBuckets(t *testing.T) {
	db := openTest(t, Retention{})
	base := time.Date(2026, 1, 1, 10, 0, 0, 0, time.UTC).Unix()
	gap := base + 10*3600

	if err := db.InsertMinute([]domain.Point{
		{Kind: domain.Temp, Name: "cpu", TS: base, Avg: 40, Min: 30, Max: 50, N: 2},
		{Kind: domain.Temp, Name: "cpu", TS: gap, Avg: 60, Min: 55, Max: 65, N: 3},
	}); err != nil {
		t.Fatalf("insert: %v", err)
	}

	hourly := history(t, db, domain.Hour)
	assertPoints(t, hourly, 2)
	assertPoint(t, hourly[0].Points[0], base, 40)
	assertStored(t, db, domain.Hour, base, 40, 30, 50, 2)
	assertPoint(t, hourly[0].Points[1], gap, 60)
	assertStored(t, db, domain.Hour, gap, 60, 55, 65, 3)

	daily := history(t, db, domain.Day)
	assertPoints(t, daily, 1)
	assertPoint(t, daily[0].Points[0], domain.Day.Bucket(base), 52)
	assertStored(t, db, domain.Day, domain.Day.Bucket(base), 52, 30, 65, 5)
}

// Pruning scans the day table and the wear log, so it runs at most hourly
// instead of on every flush. Rows older than retention may therefore linger
// for up to an hour.
func TestPruneRunsAtMostHourly(t *testing.T) {
	db := openTest(t, Retention{Minute: time.Hour, Hour: time.Hour, Day: time.Hour})
	t0 := time.Date(2026, 1, 1, 12, 0, 0, 0, time.UTC)
	at := t0
	db.now = func() time.Time { return at }
	ancient := t0.Add(-3 * time.Hour).Unix()

	if err := db.InsertMinute([]domain.Point{
		{Kind: domain.HostCPU, Name: "host", TS: ancient, Avg: 1, Min: 1, Max: 1, N: 1},
	}); err != nil {
		t.Fatal(err)
	}
	if got := minuteRows(t, db, ancient); got != 0 {
		t.Fatalf("first flush kept %d ancient rows, want 0", got)
	}

	if err := db.InsertMinute([]domain.Point{
		{Kind: domain.HostCPU, Name: "host", TS: ancient, Avg: 1, Min: 1, Max: 1, N: 1},
	}); err != nil {
		t.Fatal(err)
	}
	at = t0.Add(30 * time.Minute)
	if err := db.InsertMinute([]domain.Point{
		{Kind: domain.HostCPU, Name: "host", TS: at.Unix(), Avg: 1, Min: 1, Max: 1, N: 1},
	}); err != nil {
		t.Fatal(err)
	}
	if got := minuteRows(t, db, ancient); got != 1 {
		t.Fatalf("flush 30m later pruned again (%d ancient rows), want the hourly gate to skip", got)
	}

	at = t0.Add(61 * time.Minute)
	if err := db.InsertMinute([]domain.Point{
		{Kind: domain.HostCPU, Name: "host", TS: at.Unix(), Avg: 1, Min: 1, Max: 1, N: 1},
	}); err != nil {
		t.Fatal(err)
	}
	if got := minuteRows(t, db, ancient); got != 0 {
		t.Fatalf("flush after the hour window kept %d ancient rows, want 0", got)
	}
}

func TestWearPruneRespectsRetention(t *testing.T) {
	db := openTest(t, Retention{Wear: 24 * time.Hour})
	now := time.Date(2026, 1, 1, 12, 0, 0, 0, time.UTC)
	old := domain.WearObservation{Disk: "nvme0n1", TS: now.Add(-48 * time.Hour).Unix()}
	recent := domain.WearObservation{Disk: "nvme0n1", TS: now.Add(-time.Hour).Unix()}
	for _, w := range []domain.WearObservation{old, recent} {
		if err := db.AppendWear(w); err != nil {
			t.Fatal(err)
		}
	}
	if err := db.Prune(now); err != nil {
		t.Fatal(err)
	}
	log, err := db.WearLog(10, 0)
	if err != nil {
		t.Fatal(err)
	}
	if len(log) != 1 || log[0].TS != recent.TS {
		t.Fatalf("wear log after prune = %+v, want only the recent row", log)
	}
}

// Both hot queries must use an index; without one the minute-old prune and
// /api/wear scan whole tables.
func TestHotQueriesUseIndexes(t *testing.T) {
	db := openTest(t, Retention{})
	for _, q := range []struct{ name, query, index string }{
		{"prune days", `DELETE FROM points_day WHERE ts < 1`, "points_day_ts"},
		{"prune wear", `DELETE FROM wear_log WHERE ts < 1`, "wear_log_ts"},
		{"wear log", `SELECT ts FROM wear_log WHERE ts >= 1 ORDER BY ts DESC LIMIT 10`, "wear_log_ts"},
	} {
		rows, err := db.sql.Query(`EXPLAIN QUERY PLAN ` + q.query)
		if err != nil {
			t.Fatalf("%s: %v", q.name, err)
		}
		var plan string
		for rows.Next() {
			var id, parent, unused int
			var detail string
			if err := rows.Scan(&id, &parent, &unused, &detail); err != nil {
				t.Fatalf("%s: %v", q.name, err)
			}
			plan += detail + "\n"
		}
		rows.Close()
		if !strings.Contains(plan, q.index) {
			t.Errorf("%s does not use %s:\n%s", q.name, q.index, plan)
		}
	}
}

// A rolled back transaction must not leave cached ids that do not exist.
func TestSeriesCacheIgnoresRolledBackIDs(t *testing.T) {
	db := openTest(t, Retention{})
	key := seriesKey{kind: domain.Temp, name: "cpu:Tctl"}

	tx, err := db.sql.Begin()
	if err != nil {
		t.Fatal(err)
	}
	if _, _, err := db.resolveSeries(tx, []seriesKey{key}); err != nil {
		t.Fatalf("resolve in tx: %v", err)
	}
	if err := tx.Rollback(); err != nil {
		t.Fatal(err)
	}
	if id, ok := db.cachedID(key); ok {
		t.Fatalf("cache kept id %d from a rolled back transaction", id)
	}

	// The next real flush creates the series for real and its id must resolve.
	if err := db.InsertMinute([]domain.Point{
		{Kind: domain.Temp, Name: key.name, TS: 600, Avg: 1, Min: 1, Max: 1, N: 1},
	}); err != nil {
		t.Fatal(err)
	}
	id, ok := db.cachedID(key)
	if !ok || id == 0 {
		t.Fatalf("cache after commit = %d, %v", id, ok)
	}
	got, err := db.History([]int64{id}, 0, 1200, domain.Minute)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 || got[0].Name != key.name {
		t.Fatalf("history for cached id = %+v", got)
	}
}

// Known series must not rewrite the series table on every flush.
func TestKnownSeriesSkipsSeriesWrites(t *testing.T) {
	db := openTest(t, Retention{})
	if _, err := db.sql.Exec(`
		CREATE TABLE series_writes (n INTEGER NOT NULL);
		INSERT INTO series_writes (n) VALUES (0);
		CREATE TRIGGER count_series_write AFTER INSERT ON series
		BEGIN UPDATE series_writes SET n = n + 1; END`); err != nil {
		t.Fatalf("install write counter: %v", err)
	}
	counter := func() int {
		t.Helper()
		var n int
		if err := db.sql.QueryRow(`SELECT n FROM series_writes`).Scan(&n); err != nil {
			t.Fatalf("read counter: %v", err)
		}
		return n
	}

	row := domain.Point{Kind: domain.Temp, Name: "cpu", TS: 60, Avg: 1, Min: 1, Max: 1, N: 1}
	if err := db.InsertMinute([]domain.Point{row}); err != nil {
		t.Fatal(err)
	}
	before := counter()
	row.TS = 120
	if err := db.InsertMinute([]domain.Point{row}); err != nil {
		t.Fatal(err)
	}
	if got := counter() - before; got != 0 {
		t.Errorf("second flush wrote the series table %d times, want 0", got)
	}
}

func minuteRows(t *testing.T, db *DB, ts int64) int {
	t.Helper()
	var n int
	if err := db.sql.QueryRow(`SELECT COUNT(*) FROM points_minute WHERE ts = ?`, ts).Scan(&n); err != nil {
		t.Fatalf("count minutes: %v", err)
	}
	return n
}
