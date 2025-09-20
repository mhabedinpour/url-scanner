// Package api configures and exposes the HTTP server, routes,
// metrics, docs and related middleware for the URL Scanner service.
package api

import (
	_ "embed"
	"fmt"
	"net/http"
	"scanner/internal/api/handler/v1handler"
	"scanner/internal/api/specs/v1specs"
	"scanner/internal/config"
	"scanner/pkg/controller"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/swaggest/swgui/v5emb"
	otelprom "go.opentelemetry.io/otel/exporters/prometheus"
	sdkmetric "go.opentelemetry.io/otel/sdk/metric"
)

// v1Spec contains the embedded OpenAPI specification for version 1 of the API.
//
//go:embed specs/v1.yaml
var v1Spec []byte

// Options holds configuration for the HTTP server and its dependencies.
// It is typically created from a config.Config via NewOptions.
// All durations are used to configure server timeouts, and zero values
// should be considered as using the defaults provided by net/http where applicable.
type Options struct {
	// SecHandlerOptions configures the security handler (authn/authz) for v1 endpoints.
	SecHandlerOptions *v1handler.SecHandlerOptions

	// Addr is the TCP address the server listens on, e.g. ":8080".
	Addr string
	// ReadTimeout is the maximum duration for reading the entire request, including the body.
	ReadTimeout time.Duration
	// ReadHeaderTimeout is the amount of time allowed to read request headers.
	ReadHeaderTimeout time.Duration
	// WriteTimeout is the maximum duration before timing out writes of the response.
	WriteTimeout time.Duration
	// IdleTimeout is the maximum amount of time to wait for the next request when keep-alives are enabled.
	IdleTimeout time.Duration
	// RequestTimeout is the global timeout applied via http.TimeoutHandler for handling requests.
	RequestTimeout time.Duration
	// MaxHeaderBytes controls the maximum number of bytes the server
	// will read parsing the request header's keys and values, including the request line.
	MaxHeaderBytes int
	// MetricsPath is the HTTP path at which Prometheus metrics are served.
	MetricsPath string
}

// NewOptions constructs an Options value from the provided application configuration.
// It maps HTTP server-related settings from config.Config to the Options used by the API server.
func NewOptions(cfg *config.Config) Options {
	return Options{
		SecHandlerOptions: v1handler.NewSecHandlerOptions(cfg),

		Addr:              cfg.HTTP.Addr,
		ReadTimeout:       cfg.HTTP.ReadTimeout,
		ReadHeaderTimeout: cfg.HTTP.ReadHeaderTimeout,
		WriteTimeout:      cfg.HTTP.WriteTimeout,
		IdleTimeout:       cfg.HTTP.IdleTimeout,
		RequestTimeout:    cfg.HTTP.RequestTimeout,
		MaxHeaderBytes:    cfg.HTTP.MaxHeaderBytes,
		MetricsPath:       cfg.HTTP.MetricsPath,
	}
}

type Deps struct {
	v1handler.Deps
}

// NewServer wires up and returns a configured *http.Server using the provided Options.
// It sets up:
// - Prometheus metrics endpoint (MetricsPath)
// - OpenTelemetry metrics exporter (Prometheus)
// - Embedded OpenAPI v1 spec and Swagger UI
// - v1 API routes backed by generated server and handlers
// - pprof endpoints for profiling
// It also wraps the mux with CORS and logging middlewares and applies a request timeout.
func NewServer(deps Deps, opts Options) (*http.Server, error) {
	mux := http.NewServeMux()

	// prometheus metrics server
	mux.Handle(opts.MetricsPath, promhttp.Handler())

	// otel
	exp, err := otelprom.New(otelprom.WithRegisterer(prometheus.DefaultRegisterer))
	if err != nil {
		return nil, fmt.Errorf("could not create otel exporter: %w", err)
	}
	mp := sdkmetric.NewMeterProvider(sdkmetric.WithReader(exp))

	// v1 specs file
	mux.HandleFunc("/specs/v1.yaml", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/yaml")
		_, _ = w.Write(v1Spec)
	})
	// v1 api swagger playground
	mux.Handle("/v1/docs/", v5emb.New(
		"URL Scan Service",
		"/specs/v1.yaml",
		"/v1/docs/",
	))
	// v1 api
	secHandler, err := v1handler.NewSecHandler(opts.SecHandlerOptions)
	if err != nil {
		return nil, fmt.Errorf("could not create sec handler: %w", err)
	}
	v1Srv, err := v1specs.NewServer(v1handler.New(deps.Deps),
		secHandler,
		v1specs.WithMeterProvider(mp),
		v1specs.WithPathPrefix("/v1"))
	if err != nil {
		return nil, fmt.Errorf("could not create v1 api server: %w", err)
	}
	mux.Handle("/v1/", v1Srv)

	// pprof
	mux.Handle("/debug/pprof/", controller.PprofMux())

	// cors
	handler := controller.WithCORS(mux)

	// logger
	handler = controller.WithLogger(handler)

	return &http.Server{
		Addr:              opts.Addr,
		Handler:           http.TimeoutHandler(handler, opts.ReadTimeout, `{"error":"request timed out"}`),
		ReadTimeout:       opts.ReadTimeout,
		ReadHeaderTimeout: opts.ReadHeaderTimeout,
		WriteTimeout:      opts.WriteTimeout,
		IdleTimeout:       opts.IdleTimeout,
		MaxHeaderBytes:    opts.MaxHeaderBytes,
	}, nil
}
