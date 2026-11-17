package recorder

import (
	"testing"
	"time"

	"mon/internal/model"
)

func TestCollectOnlyClosedBuckets(t *testing.T) {
	r := New()
	base := time.Date(2026, 1, 1, 10, 0, 0, 0, time.UTC)

	r.AddAt(model.KindTemp, "cpu", 40, base)
	r.AddAt(model.KindTemp, "cpu", 50, base.Add(30*time.Second))
	r.AddAt(model.KindTemp, "cpu", 60, base.Add(time.Minute))
	r.AddAt(model.KindHostCPU, "host", 12.5, base)

	if got := r.CollectCompleted(base.Add(59 * time.Second)); len(got) != 0 {
		t.Fatalf("partial minute flushed: %+v", got)
	}
	if r.Pending() != 3 {
		t.Fatalf("pending = %d, want 3", r.Pending())
	}

	got := r.CollectCompleted(base.Add(time.Minute))
	if len(got) != 2 {
		t.Fatalf("got %+v", got)
	}
	byName := map[string]model.Point{}
	for _, p := range got {
		byName[p.Name] = p
	}
	p := byName["cpu"]
	if p.Kind != model.KindTemp || p.TS != base.Unix() || p.Avg != 45 ||
		p.Min != 40 || p.Max != 50 || p.N != 2 {
		t.Errorf("cpu point = %+v", p)
	}
	if p := byName["host"]; p.Avg != 12.5 || p.N != 1 {
		t.Errorf("host point = %+v", p)
	}
	if r.Pending() != 1 {
		t.Errorf("pending after flush = %d, want 1", r.Pending())
	}
}
