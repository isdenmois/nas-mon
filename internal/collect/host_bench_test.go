package collect

import (
	"testing"
	"time"
)

// BenchmarkHostRead is one host tick: four /proc reads plus parsing. Called
// twice a minute, so per-hour cost is 120x this number.
func BenchmarkHostRead(b *testing.B) {
	h := &Host{ProcRoot: "/proc"}
	now := time.Date(2026, 1, 2, 3, 4, 5, 0, time.UTC)

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if _, err := h.Read(now); err != nil {
			b.Fatalf("read: %v", err)
		}
	}
}
