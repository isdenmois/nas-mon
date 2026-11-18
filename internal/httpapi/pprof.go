package httpapi

import (
	"net/http"
	"net/http/pprof"
)

// PprofHandler exposes the standard Go profiles. It is only mounted on the
// opt-in profiling listener: the profiles reveal goroutine stacks, heap
// contents and command lines, so they must not be reachable from the LAN and
// are never part of Handler.
func PprofHandler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("/debug/pprof/", pprof.Index)
	mux.HandleFunc("/debug/pprof/cmdline", pprof.Cmdline)
	mux.HandleFunc("/debug/pprof/profile", pprof.Profile)
	mux.HandleFunc("/debug/pprof/symbol", pprof.Symbol)
	mux.HandleFunc("/debug/pprof/trace", pprof.Trace)
	return mux
}
