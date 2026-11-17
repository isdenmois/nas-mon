// Package recorder accumulates samples into one-minute buckets in memory and
// hands completed buckets to the store. Raw samples never touch the disk.
package recorder

import (
	"sync"
	"time"

	"mon/internal/model"
)

type key struct {
	kind string
	name string
	ts   int64
}

type bucket struct {
	kind      string
	name      string
	ts        int64
	sum, minV float64
	maxV      float64
	n         int64
}

// Recorder is safe for concurrent use by collectors.
type Recorder struct {
	mu      sync.Mutex
	buckets map[key]*bucket
	now     func() time.Time
}

// New returns an empty recorder.
func New() *Recorder {
	return &Recorder{buckets: make(map[key]*bucket), now: time.Now}
}

// Add records a sample at the current time.
func (r *Recorder) Add(kind, name string, value float64) {
	r.AddAt(kind, name, value, r.now())
}

// AddAt records a sample at ts (truncated to its minute).
func (r *Recorder) AddAt(kind, name string, value float64, ts time.Time) {
	start := ts.Unix() - ts.Unix()%60
	k := key{kind: kind, name: name, ts: start}
	r.mu.Lock()
	defer r.mu.Unlock()
	b := r.buckets[k]
	if b == nil {
		b = &bucket{kind: kind, name: name, ts: start, minV: value, maxV: value}
		r.buckets[k] = b
	}
	b.sum += value
	if value < b.minV {
		b.minV = value
	}
	if value > b.maxV {
		b.maxV = value
	}
	b.n++
}

// CollectCompleted removes and returns every bucket that is fully closed at
// now. Buckets are intentionally not flushed partially: losing the last few
// seconds on shutdown is preferable to publishing skewed averages.
func (r *Recorder) CollectCompleted(now time.Time) []model.Point {
	current := now.Unix() - now.Unix()%60
	r.mu.Lock()
	defer r.mu.Unlock()
	var out []model.Point
	for k, b := range r.buckets {
		if b.ts >= current {
			continue
		}
		out = append(out, model.Point{
			Kind: b.kind, Name: b.name, TS: b.ts,
			Avg: b.sum / float64(b.n), Min: b.minV, Max: b.maxV, N: b.n,
		})
		delete(r.buckets, k)
	}
	return out
}

// Pending reports how many buckets are buffered (used by tests and metrics).
func (r *Recorder) Pending() int {
	r.mu.Lock()
	defer r.mu.Unlock()
	return len(r.buckets)
}
