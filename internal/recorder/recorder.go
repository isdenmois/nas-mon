// Package recorder accumulates samples into one-minute buckets in memory and
// hands completed buckets to the store. Raw samples never touch the disk.
package recorder

import (
	"sync"
	"time"

	"mon/internal/domain"
)

type key struct {
	kind domain.Kind
	name string
	ts   int64
}

type bucket struct {
	kind      domain.Kind
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
func (r *Recorder) Add(kind domain.Kind, name string, value float64) bool {
	return r.AddAt(kind, name, value, r.now())
}

// AddAt records a sample at ts (truncated to its minute). Non-finite values are
// rejected at this boundary and never reach a bucket.
func (r *Recorder) AddAt(kind domain.Kind, name string, value float64, ts time.Time) bool {
	value, ok := domain.Sanitize(value)
	if !ok {
		return false
	}
	start := domain.Minute.Bucket(ts.Unix())
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
	return true
}

// CollectCompleted removes and returns every bucket that is fully closed at
// now. Buckets are intentionally not flushed partially: losing the last few
// seconds on shutdown is preferable to publishing skewed averages.
func (r *Recorder) CollectCompleted(now time.Time) []domain.Point {
	current := domain.Minute.Bucket(now.Unix())
	r.mu.Lock()
	defer r.mu.Unlock()
	var out []domain.Point
	for k, b := range r.buckets {
		if b.ts >= current {
			continue
		}
		out = append(out, domain.Point{
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
