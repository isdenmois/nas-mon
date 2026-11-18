package store

import (
	"fmt"
	"path/filepath"
	"testing"
	"time"

	"mon/internal/domain"
)

const benchSeries = 120

func benchDB(b *testing.B, ret Retention) *DB {
	b.Helper()
	db, err := Open(filepath.Join(b.TempDir(), "bench.db"), ret)
	if err != nil {
		b.Fatalf("open: %v", err)
	}
	b.Cleanup(func() { db.Close() })
	return db
}

// pointsFor returns one minute of samples for benchSeries series.
func pointsFor(ts int64) []domain.Point {
	pts := make([]domain.Point, 0, benchSeries)
	for i := 0; i < benchSeries; i++ {
		pts = append(pts, domain.Point{
			Kind: domain.ContainerCPU, Name: fmt.Sprintf("container-%03d", i),
			TS: ts, Avg: 1.5, Min: 0.5, Max: 3.5, N: 2,
		})
	}
	return pts
}

// BenchmarkFlushOneMinute is the old cadence: every tick carries one minute of
// samples for every series.
func BenchmarkFlushOneMinute(b *testing.B) {
	db := benchDB(b, Retention{})
	base := time.Date(2026, 1, 2, 0, 0, 0, 0, time.UTC).Unix()
	if err := db.InsertMinute(pointsFor(base)); err != nil {
		b.Fatalf("seed: %v", err)
	}
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		ts := base + int64(i+1)*60
		if err := db.InsertMinute(pointsFor(ts)); err != nil {
			b.Fatalf("flush: %v", err)
		}
	}
}

// BenchmarkFlushFiveMinutes is the new cadence: the same samples, batched.
func BenchmarkFlushFiveMinutes(b *testing.B) {
	db := benchDB(b, Retention{})
	base := time.Date(2026, 1, 2, 0, 0, 0, 0, time.UTC).Unix()
	if err := db.InsertMinute(pointsFor(base)); err != nil {
		b.Fatalf("seed: %v", err)
	}
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		batch := make([]domain.Point, 0, benchSeries*5)
		for m := 0; m < 5; m++ {
			batch = append(batch, pointsFor(base+int64(i*5+m+1)*60)...)
		}
		if err := db.InsertMinute(batch); err != nil {
			b.Fatalf("flush: %v", err)
		}
	}
}

// BenchmarkPruneDay measures the every-flush retention scan against a table
// with the five years of daily rows a long-running instance accumulates.
func BenchmarkPruneDay(b *testing.B) {
	db := benchDB(b, Retention{Day: 5 * 365 * 24 * time.Hour})
	tx, err := db.sql.Begin()
	if err != nil {
		b.Fatal(err)
	}
	stmt, err := tx.Prepare(`INSERT INTO points_day (series_id, ts, avg, min, max, n)
		VALUES (?, ?, 1, 1, 1, 1)`)
	if err != nil {
		b.Fatal(err)
	}
	start := time.Now().Add(-6 * 365 * 24 * time.Hour).Truncate(24 * time.Hour)
	for s := 0; s < benchSeries; s++ {
		for d := 0; d < 6*365; d++ {
			if _, err := stmt.Exec(s+1, start.Add(time.Duration(d)*24*time.Hour).Unix()); err != nil {
				b.Fatal(err)
			}
		}
	}
	stmt.Close()
	if err := tx.Commit(); err != nil {
		b.Fatal(err)
	}
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if err := db.Prune(time.Now()); err != nil {
			b.Fatalf("prune: %v", err)
		}
	}
}

// BenchmarkHistory24h is one charts-page request: 24 hours at minute
// resolution for 25 series (the container panels), the shape a single open
// dashboard tab asks for eight times a minute.
func BenchmarkHistory24h(b *testing.B) {
	db := benchDB(b, Retention{})
	base := time.Date(2026, 1, 2, 0, 0, 0, 0, time.UTC).Unix()
	for m := 0; m < 24*60; m++ {
		if err := db.InsertMinute(pointsFor(base + int64(m)*60)); err != nil {
			b.Fatalf("seed: %v", err)
		}
	}
	ids := make([]int64, 0, 25)
	for i := 1; i <= 25; i++ {
		ids = append(ids, int64(i))
	}
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		got, err := db.History(ids, base, base+24*3600, domain.Minute)
		if err != nil {
			b.Fatalf("history: %v", err)
		}
		if len(got) != 25 {
			b.Fatalf("series = %d, want 25", len(got))
		}
	}
}
