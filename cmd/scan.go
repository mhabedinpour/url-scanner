package main

import (
	"context"
	"errors"
	"net/http"
	"os/signal"
	"scanner/internal/api"
	"scanner/internal/api/handler/v1handler"
	"scanner/internal/config"
	"scanner/internal/scanner"
	"scanner/internal/worker"
	"scanner/pkg/logger"
	"syscall"

	"github.com/spf13/cobra"
	"go.uber.org/zap"
)

// setupServer configures and starts the HTTP server asynchronously and returns
// a function that gracefully shuts it down using the provided context.
func setupServer(ctx context.Context, cfg *config.Config, deps api.Deps) func(ctx context.Context) {
	server, err := api.NewServer(ctx, deps, api.NewOptions(cfg))
	if err != nil {
		logger.Fatal(ctx, "could not create webserver", zap.Error(err))
	}

	go func() {
		logger.Info(ctx, "starting webserver...")
		if err := server.ListenAndServe(); err != nil {
			if !errors.Is(err, http.ErrServerClosed) {
				logger.Error(ctx, "could not start webserver", zap.Error(err))
			}
		}
	}()

	return func(ctx context.Context) {
		logger.Info(ctx, "stopping webserver...")
		if err := server.Shutdown(ctx); err != nil {
			logger.Error(ctx, "could not stop webserver", zap.Error(err))
		}
	}
}

// scanCommand constructs the 'scan' subcommand that runs the API server and
// background workers until interrupted.
func scanCommand(cfg *config.Config) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "scan",
		Short: "Starts API server and background workers",
		Run: func(cmd *cobra.Command, args []string) {
			ctx, _ := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)

			strg, closeStrg := getPostgres(ctx, cfg)
			defer closeStrg()

			workerClient, err := worker.Start(ctx, strg.Pool)
			if err != nil {
				logger.Fatal(ctx, "could not start worker", zap.Error(err))
			}

			stopWebserver := setupServer(ctx, cfg, api.Deps{
				Deps: v1handler.Deps{
					Scanner: scanner.New(strg, scanner.NewOptions(cfg)),
				},
				WorkerClient: workerClient,
			})

			// wait for interrupt
			<-ctx.Done()
			shutdownCtx, cancel := context.WithTimeout(context.Background(), cfg.GracefulShutdownTimeout)
			defer cancel()

			stopWebserver(shutdownCtx)

			logger.Info(ctx, "stopping worker...")
			if err := workerClient.Stop(shutdownCtx); err != nil {
				logger.Warn(ctx, "could not stop worker", zap.Error(err))
			}
		},
	}

	return cmd
}
