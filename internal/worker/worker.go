package worker

import (
	"context"
	"fmt"
	"log/slog"
	"scanner/pkg/logger"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/riverqueue/river"
	"github.com/riverqueue/river/riverdriver/riverpgxv5"
	"go.uber.org/zap/exp/zapslog"
)

func Start(ctx context.Context, dbPool *pgxpool.Pool) (*river.Client[pgx.Tx], error) {
	riverClient, err := river.NewClient(riverpgxv5.New(dbPool), &river.Config{
		// Queues: map[string]river.QueueConfig{
		//	river.QueueDefault: {MaxWorkers: 100},
		// },
		// ErrorHandler
		// Workers
		Logger: slog.New(zapslog.NewHandler(logger.Get(ctx).Core())),
	})
	if err != nil {
		return nil, fmt.Errorf("could not create river queue client: %w", err)
	}

	return riverClient, nil
}
