// Package worker wires up and runs background workers that process queued jobs
// using the river queue backed by PostgreSQL. It exposes helpers to build
// options from configuration and to start the worker client.
package worker

import (
	"context"
	"fmt"
	"log/slog"
	"scanner/internal/config"
	"scanner/internal/scanner"
	"scanner/pkg/logger"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/riverqueue/river"
	"github.com/riverqueue/river/riverdriver/riverpgxv5"
	"go.uber.org/zap/exp/zapslog"
)

// Options contain runtime settings for the background workers.
type Options struct {
	// JobTimeout controls the maximum duration a single job is allowed to run
	// before it is considered timed out by river.
	JobTimeout time.Duration
	// JobConcurrency specifies how many jobs can be processed in parallel.
	JobConcurrency int
}

// NewOptions translates the application's config into worker Options.
// It copies relevant values from cfg.Worker so callers can pass them to Start.
func NewOptions(cfg *config.Config) Options {
	return Options{
		JobTimeout:     cfg.Worker.JobTimeout,
		JobConcurrency: cfg.Worker.JobConcurrency,
	}
}

// Start initializes the river client, registers workers, and starts processing
// jobs. It returns the started river client which should be closed by the
// caller when shutting down (by canceling ctx).
func Start(
	ctx context.Context,
	dbPool *pgxpool.Pool,
	scanner scanner.Scanner,
	options Options,
) (*river.Client[pgx.Tx], error) {
	workers := river.NewWorkers()
	river.AddWorker(workers, NewURLScannerWorker(scanner))

	riverClient, err := river.NewClient(riverpgxv5.New(dbPool), &river.Config{
		Queues: map[string]river.QueueConfig{
			river.QueueDefault: {MaxWorkers: options.JobConcurrency},
		},
		JobTimeout: options.JobTimeout,
		Workers:    workers,
		Logger:     slog.New(zapslog.NewHandler(logger.Get(ctx).Core())),
	})
	if err != nil {
		return nil, fmt.Errorf("could not create river queue client: %w", err)
	}

	if err := riverClient.Start(ctx); err != nil {
		return nil, fmt.Errorf("could not start river queue client: %w", err)
	}

	return riverClient, nil
}
