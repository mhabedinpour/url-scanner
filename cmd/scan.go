package main

import (
	"context"
	"errors"
	"net/http"
	"os/signal"
	"scanner/internal/api"
	"scanner/internal/config"
	"scanner/pkg/logger"
	"syscall"

	"github.com/spf13/cobra"
	"go.uber.org/zap"
)

func setupServer(ctx context.Context, cfg *config.Config) func(ctx context.Context) {
	server, err := api.NewServer(api.Options{
		Addr:              cfg.HTTP.Addr,
		ReadTimeout:       cfg.HTTP.ReadTimeout,
		ReadHeaderTimeout: cfg.HTTP.ReadHeaderTimeout,
		WriteTimeout:      cfg.HTTP.WriteTimeout,
		IdleTimeout:       cfg.HTTP.IdleTimeout,
		RequestTimeout:    cfg.HTTP.RequestTimeout,
		MaxHeaderBytes:    cfg.HTTP.MaxHeaderBytes,
		MetricsPath:       cfg.HTTP.MetricsPath,
	})
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

func scanCommand(cfg *config.Config) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "scan",
		Short: "Starts API server and background workers",
		Run: func(cmd *cobra.Command, args []string) {
			ctx, _ := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)

			stopWebserver := setupServer(ctx, cfg)

			_, closeStrg := getPostgres(ctx, cfg)
			defer closeStrg()

			// wait for interrupt
			<-ctx.Done()
			shutdownCtx, cancel := context.WithTimeout(context.Background(), cfg.GracefulShutdownTimeout)
			defer cancel()

			stopWebserver(shutdownCtx)
		},
	}

	return cmd
}
