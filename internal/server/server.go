// Package server exposes the dashboard API, Prometheus metrics and the
// embedded web UI.
package server

import (
	"encoding/json"
	"errors"
	"io/fs"
	"log/slog"
	"net/http"
	"strconv"
	"strings"
	"time"

	"mon/internal/model"
	"mon/internal/state"
	"mon/internal/store"
)

// Store is the subset of the SQLite store used by HTTP handlers.
type Store interface {
	History(kind string, names []string, from, to int64, step store.Step) ([]model.SeriesHistory, error)
	WearLatest() ([]store.Wear, error)
	WearLog(limit int) ([]store.Wear, error)
}

// Server holds handler dependencies.
type Server struct {
	state *state.State
	store Store
	log   *slog.Logger
	now   func() time.Time
	dist  fs.FS
}

// New creates the HTTP server. dist is the built frontend (usually the
// embedded directory); it may be nil in tests.
func New(st *state.State, db Store, log *slog.Logger) *Server {
	if log == nil {
		log = slog.Default()
	}
	s := &Server{state: st, store: db, log: log, now: time.Now}
	sub, err := fs.Sub(distFS, "dist")
	if err == nil {
		s.dist = sub
	}
	return s
}

// Handler returns the fully wired HTTP handler.
func (s *Server) Handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /healthz", s.handleHealth)
	mux.HandleFunc("GET /api/overview", s.handleOverview)
	mux.HandleFunc("GET /api/history", s.handleHistory)
	mux.HandleFunc("GET /api/wear", s.handleWear)
	mux.HandleFunc("GET /metrics", s.handleMetrics)
	mux.Handle("/", s.staticHandler())
	return s.withHeaders(mux)
}

func (s *Server) handleHealth(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, map[string]any{"ok": true})
}

func (s *Server) handleOverview(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, s.state.Snapshot())
}

func (s *Server) handleHistory(w http.ResponseWriter, r *http.Request) {
	raw := r.URL.Query()["series"]
	if len(raw) == 0 {
		writeError(w, http.StatusBadRequest, "missing series parameter")
		return
	}
	groups := map[string][]string{}
	var order []string
	for _, item := range raw {
		for _, part := range strings.Split(item, ",") {
			kind, name, ok := strings.Cut(strings.TrimSpace(part), ":")
			if !ok || kind == "" || name == "" {
				writeError(w, http.StatusBadRequest, "series must look like kind:name")
				return
			}
			if _, seen := groups[kind]; !seen {
				order = append(order, kind)
			}
			groups[kind] = append(groups[kind], name)
		}
	}

	from, to, step, err := resolveRange(r.URL.Query().Get("range"), s.now())
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	out := make([]model.SeriesHistory, 0, len(raw))
	for _, kind := range order {
		h, err := s.store.History(kind, groups[kind], from, to, step)
		if err != nil {
			s.log.Warn("history query failed", "kind", kind, "err", err)
			writeError(w, http.StatusInternalServerError, "history query failed")
			return
		}
		out = append(out, h...)
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"step":   step,
		"from":   from,
		"to":     to,
		"series": out,
	})
}

func (s *Server) handleWear(w http.ResponseWriter, r *http.Request) {
	limit := 200
	if raw := r.URL.Query().Get("limit"); raw != "" {
		v, err := strconv.Atoi(raw)
		if err != nil || v < 1 || v > 1000 {
			writeError(w, http.StatusBadRequest, "limit must be 1..1000")
			return
		}
		limit = v
	}
	latest, err := s.store.WearLatest()
	if err != nil {
		s.log.Warn("wear latest failed", "err", err)
		writeError(w, http.StatusInternalServerError, "wear query failed")
		return
	}
	log, err := s.store.WearLog(limit)
	if err != nil {
		s.log.Warn("wear log failed", "err", err)
		writeError(w, http.StatusInternalServerError, "wear query failed")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"latest": latest, "log": log})
}

// resolveRange maps a UI range preset to a time window and aggregation table.
func resolveRange(raw string, now time.Time) (from, to int64, step store.Step, err error) {
	to = now.Unix()
	switch raw {
	case "1h", "6h", "24h", "48h":
		step = store.StepMinute
	case "7d", "30d", "90d":
		step = store.StepHour
	case "1y", "all":
		step = store.StepDay
	default:
		return 0, 0, "", errors.New("range must be one of 1h, 6h, 24h, 48h, 7d, 30d, 90d, 1y, all")
	}
	windows := map[string]time.Duration{
		"1h": time.Hour, "6h": 6 * time.Hour, "24h": 24 * time.Hour,
		"48h": 48 * time.Hour, "7d": 7 * 24 * time.Hour,
		"30d": 30 * 24 * time.Hour, "90d": 90 * 24 * time.Hour,
		"1y": 365 * 24 * time.Hour,
	}
	if raw == "all" {
		from = 0
	} else {
		from = to - int64(windows[raw].Seconds())
	}
	from -= from % int64(stepSeconds(step))
	return from, to, step, nil
}

func stepSeconds(step store.Step) int64 {
	switch step {
	case store.StepMinute:
		return 60
	case store.StepHour:
		return 3600
	default:
		return 86400
	}
}

func (s *Server) staticHandler() http.Handler {
	files := http.FileServer(http.FS(s.dist))
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.HasPrefix(r.URL.Path, "/api/") || r.URL.Path == "/api" {
			if r.Method != http.MethodGet && r.Method != http.MethodHead {
				writeError(w, http.StatusMethodNotAllowed, "method not allowed")
				return
			}
			writeError(w, http.StatusNotFound, "not found")
			return
		}
		if s.dist == nil {
			http.Error(w, "frontend not built", http.StatusNotFound)
			return
		}
		p := strings.TrimPrefix(r.URL.Path, "/")
		if p == "" || !fileExists(s.dist, p) {
			r = r.Clone(r.Context())
			r.URL.Path = "/"
		}
		if strings.HasPrefix(r.URL.Path, "/assets/") {
			w.Header().Set("Cache-Control", "public, max-age=31536000, immutable")
		} else {
			w.Header().Set("Cache-Control", "no-store")
		}
		files.ServeHTTP(w, r)
	})
}

func fileExists(fsys fs.FS, name string) bool {
	st, err := fs.Stat(fsys, name)
	return err == nil && !st.IsDir()
}

func (s *Server) withHeaders(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("Referrer-Policy", "same-origin")
		if strings.HasPrefix(r.URL.Path, "/api/") || r.URL.Path == "/metrics" {
			w.Header().Set("Cache-Control", "no-store")
		}
		next.ServeHTTP(w, r)
	})
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(v); err != nil {
		slog.Debug("write JSON", "err", err)
	}
}

func writeError(w http.ResponseWriter, status int, msg string) {
	writeJSON(w, status, map[string]string{"error": msg})
}
