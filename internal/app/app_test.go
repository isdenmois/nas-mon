package app

import (
	"io"
	"log/slog"
	"path/filepath"
	"testing"
	"time"

	"mon/internal/domain"
	"mon/internal/recorder"
	"mon/internal/store"
)

func TestFlushWritesClosedMinutesOnlyOnce(t *testing.T) {
	db, err := store.Open(filepath.Join(t.TempDir(), "mon.db"), 0, 0)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	a := &App{db: db, rec: recorder.New(), log: slog.New(slog.NewTextHandler(io.Discard, nil))}

	now := time.Date(2026, 3, 4, 12, 0, 30, 0, time.UTC)
	a.rec.AddAt(domain.Temp, "cpu", 40, now.Add(-2*time.Minute))
	a.rec.AddAt(domain.Temp, "cpu", 50, now.Add(-2*time.Minute+10*time.Second))
	a.rec.AddAt(domain.Temp, "cpu", 99, now) // current minute, must stay buffered

	if err := a.flush(now); err != nil {
		t.Fatal(err)
	}
	from := now.Add(-time.Hour).Unix()
	got, err := db.History(domain.Temp, []string{"cpu"}, from, now.Unix()+60, domain.Minute)
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
	got, _ = db.History(domain.Temp, []string{"cpu"}, from, now.Unix()+60, domain.Minute)
	if len(got[0].Points) != 1 {
		t.Errorf("second flush duplicated rows: %+v", got)
	}
}
