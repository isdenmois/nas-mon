package recorder

import (
	"math"
	"testing"
	"time"

	"mon/internal/domain"
)

func TestCollectOnlyClosedBuckets(t *testing.T) {
	r := New()
	base := time.Date(2026, 1, 1, 10, 0, 0, 0, time.UTC)

	r.AddAt(domain.Temp, "cpu", 40, base)
	r.AddAt(domain.Temp, "cpu", 50, base.Add(30*time.Second))
	r.AddAt(domain.Temp, "cpu", 60, base.Add(time.Minute))
	r.AddAt(domain.HostCPU, "host", 12.5, base)

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
	byName := map[string]domain.Point{}
	for _, p := range got {
		byName[p.Name] = p
	}
	p := byName["cpu"]
	if p.Kind != domain.Temp || p.TS != base.Unix() || p.Avg != 45 ||
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

func TestRejectsNonFiniteSamples(t *testing.T) {
	r := New()
	base := time.Date(2026, 1, 1, 10, 0, 0, 0, time.UTC)

	for _, v := range []float64{math.NaN(), math.Inf(1), math.Inf(-1)} {
		if r.AddAt(domain.Temp, "cpu", v, base) {
			t.Errorf("AddAt(%v) accepted", v)
		}
	}
	if r.Pending() != 0 {
		t.Errorf("pending = %d, want 0", r.Pending())
	}
}
