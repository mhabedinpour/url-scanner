// Package controller contains HTTP middlewares and helper handlers used by the API server.
//
// Provided middlewares:
//   - WithCORS: Adds permissive CORS headers and handles OPTIONS preflight.
//   - WithLogger: Attaches a request-scoped logger and request ID to the context and logs access info.
//
// Provided helpers:
//   - PprofMux: Returns a ServeMux exposing net/http/pprof handlers.
package controller
