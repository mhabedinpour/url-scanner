package api

import (
	_ "embed"
	"fmt"
	"net/http"
	"scanner/internal/api/handler/v1handler"
	"scanner/internal/api/specs/v1specs"
	"scanner/pkg/controller"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/swaggest/swgui/v5emb"
	otelprom "go.opentelemetry.io/otel/exporters/prometheus"
	sdkmetric "go.opentelemetry.io/otel/sdk/metric"
)

//go:embed specs/v1.yaml
var v1Spec []byte

type Options struct {
	Addr              string
	ReadTimeout       time.Duration
	ReadHeaderTimeout time.Duration
	WriteTimeout      time.Duration
	IdleTimeout       time.Duration
	RequestTimeout    time.Duration
	MaxHeaderBytes    int
	MetricsPath       string
}

func NewServer(opts Options) (*http.Server, error) {
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
	v1Srv, err := v1specs.NewServer(v1handler.New(),
		v1handler.NewSecHandler(),
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
