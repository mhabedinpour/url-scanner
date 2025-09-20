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
	"scanner/pkg/logger"
	"scanner/pkg/storage"
	"syscall"

	"github.com/spf13/cobra"
	"go.uber.org/zap"
)

// setupServer configures and starts the HTTP server asynchronously and returns
// a function that gracefully shuts it down using the provided context.
func setupServer(ctx context.Context, cfg *config.Config, strg storage.Storage) func(ctx context.Context) {
	server, err := api.NewServer(api.Deps{
		Deps: v1handler.Deps{
			Scanner: scanner.New(strg),
		},
	}, api.NewOptions(cfg))
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

			stopWebserver := setupServer(ctx, cfg, strg)

			// wait for interrupt
			<-ctx.Done()
			shutdownCtx, cancel := context.WithTimeout(context.Background(), cfg.GracefulShutdownTimeout)
			defer cancel()

			stopWebserver(shutdownCtx)
		},
	}

	return cmd
}
