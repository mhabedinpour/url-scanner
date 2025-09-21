package worker

import (
	"context"
	"errors"
	"fmt"
	"scanner/internal/scanner"
	"scanner/pkg/logger"
	"scanner/pkg/serrors"

	"github.com/riverqueue/river"
	"go.uber.org/zap"
)

type URLScannerWorker struct {
	river.WorkerDefaults[scanner.JobArgs]

	scanner scanner.Scanner
}

// TODO: tests and docs

func (u *URLScannerWorker) Work(ctx context.Context, job *river.Job[scanner.JobArgs]) error {
	ctx = logger.WithFields(ctx, zap.Int64("jobID", job.ID), zap.String("URL", job.Args.URL))
	if err := u.scanner.Scan(ctx, job.Args.URL); err != nil {
		if errors.Is(err, serrors.ErrConflict) {
			return river.JobCancel(err) //nolint: wrapcheck
		}

		logger.Error(ctx, "error in scanning URL", zap.Error(err))

		return fmt.Errorf("could not scan URL: %w", err)
	}

	// TODO: transactional updates

	logger.Info(ctx, "URL scanned successfully")

	return nil
}
