package httpapi

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestPprofHandlerServesProfiles(t *testing.T) {
	// Arrange
	srv := PprofHandler()

	// Act
	rec := httptest.NewRecorder()
	srv.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/debug/pprof/", nil))

	// Assert
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	if body := rec.Body.String(); !strings.Contains(body, "goroutine") {
		t.Errorf("pprof index does not list profiles:\n%s", body)
	}
}

// The profiler exposes goroutine stacks and heap contents, so it must never be
// reachable on the dashboard port, not even through the SPA fallback.
func TestMainHandlerDoesNotServeProfiles(t *testing.T) {
	// Arrange
	srv := New(&fakeMonitor{}, nil).Handler()

	// Act
	rec := httptest.NewRecorder()
	srv.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/debug/pprof/", nil))

	// Assert
	body := rec.Body.String()
	for _, leak := range []string{"Types of profiles available", "goroutine profile", "heap profile"} {
		if strings.Contains(body, leak) {
			t.Errorf("dashboard port served %q from the pprof index", leak)
		}
	}
}
