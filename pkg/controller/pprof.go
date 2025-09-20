package controller

import (
	"net/http"
	"net/http/pprof"
)

// PprofMux returns an http.ServeMux with net/http/pprof handlers registered
// at the root. It can be mounted under a debug path in the main HTTP server.
func PprofMux() *http.ServeMux {
	mux := http.NewServeMux()

	mux.HandleFunc("/", pprof.Index)
	mux.HandleFunc("/cmdline", pprof.Cmdline)
	mux.HandleFunc("/profile", pprof.Profile)
	mux.HandleFunc("/symbol", pprof.Symbol)
	mux.HandleFunc("/trace", pprof.Trace)

	return mux
}
