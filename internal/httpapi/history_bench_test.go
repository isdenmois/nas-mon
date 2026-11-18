package httpapi

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"mon/internal/api"
)

// BenchmarkHistoryHandler24h encodes one charts-page answer: 25 series over 24
// hours at minute resolution. bytes/op is the response body on the wire.
func BenchmarkHistoryHandler24h(b *testing.B) {
	const base = int64(1767225600)
	histories := make([]api.SeriesHistory, 0, 25)
	for i := 0; i < 25; i++ {
		points := make([]api.SeriesPoint, 0, 24*60)
		for m := 0; m < 24*60; m++ {
			points = append(points, api.SeriesPoint{
				TS:  base + int64(m)*60,
				Avg: 10 + float64(m)/100,
			})
		}
		histories = append(histories, api.SeriesHistory{
			ID: int64(i + 1), Kind: "container_cpu",
			Name: fmt.Sprintf("svc-%02d", i), Points: points,
		})
	}
	srv := New(&fakeMonitor{histories: histories}, nil).Handler()
	req := httptest.NewRequest(http.MethodGet, "/api/history?id=1,2,3&range=24h", nil)

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		rec := httptest.NewRecorder()
		srv.ServeHTTP(rec, req)
		if rec.Code != http.StatusOK {
			b.Fatalf("status = %d", rec.Code)
		}
		b.SetBytes(int64(rec.Body.Len()))
	}
}
