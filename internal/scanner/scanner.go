package scanner

import (
	"context"
	"errors"
	"fmt"
	"scanner/internal/config"
	"scanner/pkg/domain"
	"scanner/pkg/logger"
	"scanner/pkg/serrors"
	"scanner/pkg/storage"
	"scanner/pkg/urlscanner"
	"time"

	"go.uber.org/zap"
)

const (
	scanResultPollTimeout      = 30 * time.Second
	scanResultPollInitialDelay = time.Second
	scanResultPollIntervalBase = 2 * time.Second
	scanResultPollIntervalMax  = 10 * time.Minute
)

// Options configure how scan jobs are enqueued and how results are cached.
// These settings are typically derived from application configuration.
type Options struct {
	// MaxAttempts is the maximum number of attempts the background worker should
	// make when processing a scan job before marking it failed.
	MaxAttempts int
	// ResultCacheTTL is the duration during which a completed result makes new
	// scan requests for the same URL reuse that result instead of enqueueing
	// a duplicate job.
	ResultCacheTTL time.Duration
}

// NewOptions constructs an Options value from the provided application config.
func NewOptions(cfg *config.Config) Options {
	return Options{
		MaxAttempts:    cfg.Scanner.MaxAttempts,
		ResultCacheTTL: cfg.Scanner.ResultCacheTTL,
	}
}

// scanner is the concrete implementation of the Scanner interface.
// It coordinates persistence with the storage layer and job enqueueing.
type scanner struct {
	// options holds runtime configuration that affects enqueueing and caching.
	options Options
	// storage is the persistence layer used to store scans and manage jobs.
	storage storage.Storage
	// urlScanner is the client used to submit scan requests to urlscan.io.
	urlScanner urlscanner.Client
}

// Enqueue stores a new scan request for the given URL and user, and attempts
// to enqueue a background job to process it. If a recent completed result exists
// for the same URL (within ResultCacheTTL), the new scan is immediately marked
// as completed with that result.
func (s scanner) Enqueue(ctx context.Context, userID domain.UserID, URL string) (*domain.Scan, error) {
	var scan *domain.Scan
	URL, err := NormalizeURL(URL)
	if err != nil {
		return nil, serrors.Wrap(serrors.ErrBadRequest, err, "invalid URL")
	}

	if err := s.storage.WithTx(ctx, func(tx storage.AllStorage) error {
		res, err := tx.StoreScans(ctx, domain.Scan{
			UserID: userID,
			URL:    URL,
			Status: domain.ScanStatusPending,
		})
		if err != nil {
			return fmt.Errorf("could not store scan: %w", err)
		}
		scan = &res[0]

		jobAdded, err := tx.AddJob(ctx, JobArgs{
			URL:             URL,
			maxAttempts:     s.options.MaxAttempts,
			uniqueJobPeriod: s.options.ResultCacheTTL,
		}, nil)
		if err != nil {
			return fmt.Errorf("could not add job: %w", err)
		}

		// if a job was not added, it means that another job already exists for this URL.
		// river unique jobs prevent having duplicate jobs for the same URL.
		if !jobAdded {
			// if existing jobs is already completed, we should get its result from db and
			// update the new scan
			lastResult, err := tx.LastCompletedScanByURL(ctx, URL)
			if err != nil {
				return fmt.Errorf("could not get last completed scan: %w", err)
			}

			if lastResult != nil {
				updated, err := tx.UpdateScanByID(ctx, scan.ID, storage.ScanUpdates{
					Status: domain.ScanStatusCompleted,
					Result: &lastResult.Result,
				})
				if err != nil {
					return fmt.Errorf("could not update scan: %w", err)
				}
				scan = updated
			} // else: the job is in the queue and will be processed soon.
			// Job will automatically update all pending jobs by URL upon completion.
		}

		return nil
	}); err != nil {
		return nil, fmt.Errorf("could not enqueue URL: %w", err)
	}

	return scan, nil
}

// UserScans returns a page of scans for the given user filtered by status.
// It supports cursor-based pagination using an RFC3339 timestamp string and
// returns the next cursor when more results are available.
func (s scanner) UserScans(ctx context.Context,
	userID domain.UserID,
	status domain.ScanStatus,
	cursor string,
	limit uint) ([]domain.Scan, string, error) {
	var cursorTime time.Time
	if cursor != "" {
		t, err := time.Parse(time.RFC3339, cursor)
		if err != nil {
			return nil, "", serrors.Wrap(serrors.ErrBadRequest, err, "invalid cursor")
		}
		cursorTime = t
	}

	page, err := s.storage.UserScans(ctx, userID, status, cursorTime, limit)
	if err != nil {
		return nil, "", fmt.Errorf("could not get user scans: %w", err)
	}

	var next string
	if page.NextCursor != nil {
		next = page.NextCursor.Format(time.RFC3339)
	}

	return page.Scans, next, nil
}

// Result fetches a single scan by ID for the given user. It returns a
// not-found error when no matching scan exists.
func (s scanner) Result(ctx context.Context, userID domain.UserID, scanID domain.ScanID) (*domain.Scan, error) {
	res, err := s.storage.ScanByID(ctx, userID, scanID)
	if err != nil {
		return nil, fmt.Errorf("could not get scan results: %w", err)
	}
	if res == nil {
		return nil, serrors.With(serrors.ErrNotFound, "scan not found")
	}

	return res, nil
}

// Delete removes a scan belonging to the given user. If the scan does not
// exist, a not-found error is returned. Jobs are not cancelled here because
// other pending scans may still depend on the same URL job.
func (s scanner) Delete(ctx context.Context, userID domain.UserID, scanID domain.ScanID) error {
	res, err := s.storage.DeleteScan(ctx, userID, scanID)
	if err != nil {
		return fmt.Errorf("could not delete scan: %w", err)
	}
	if res == nil {
		return serrors.With(serrors.ErrNotFound, "scan not found")
	}

	// we don't delete jobs from the queue here because there might be other scans depending on the job.
	// job worker makes sure there are still pending scans for the URL before processing.

	return nil
}

// Scan processes all pending scans for the given URL.
//
// It first verifies there are still pending scans for the URL (to avoid
// running orphaned jobs after a user deletes their scan request). If there are
// none, it returns a conflict error and exits without side effects.
//
// When pending scans exist, Scan submits the URL to the external urlscanner
// provider and waits for the result. On success, it marks all pending scans for
// the URL as completed and stores the result. On failure, it marks them as
// failed with the last error recorded.
//
// The method returns the provider's urlscanner.RateLimitStatus to allow
// callers (e.g., background workers) to adjust scheduling/backoff according to
// rate limiting information.
//
// This method is designed to be invoked by a background worker and to be
// idempotent with respect to concurrently deleted scan requests.
func (s scanner) Scan(ctx context.Context, URL string) (urlscanner.RateLimitStatus, error) {
	// makes sure there are still pending scans for the URL before processing,
	// this is required because during scan deletion we do not cancel jobs
	pendingCount, err := s.storage.PendingScanCountByURL(ctx, URL)
	if err != nil {
		return urlscanner.RateLimitStatus{}, fmt.Errorf("could not get pending scan count: %w", err)
	}
	if pendingCount <= 0 {
		logger.Warn(ctx, "no pending scans for URL, skipping")

		return urlscanner.RateLimitStatus{}, serrors.With(serrors.ErrConflict, "no pending scans for URL")
	}

	res, RLStatus, err := s.submitURLAndPoll(ctx, URL)
	if err != nil {
		if !errors.Is(err, serrors.ErrRateLimited) {
			lastErr := err.Error()
			if err := s.storage.UpdatePendingScansByURL(ctx, URL, storage.ScanUpdates{
				Status:      domain.ScanStatusFailed,
				LastError:   &lastErr,
				MaxAttempts: s.options.MaxAttempts,
			}); err != nil {
				// just log the error and continue
				logger.Error(ctx, "error updating scan", zap.Error(err))
			}
		}

		return RLStatus, err
	}

	if err := s.storage.UpdatePendingScansByURL(ctx, URL, storage.ScanUpdates{
		Status: domain.ScanStatusCompleted,
		Result: res,
	}); err != nil {
		return RLStatus, fmt.Errorf("could not update scan: %w", err)
	}

	return RLStatus, nil
}

// submitURLAndPoll submits the URL to the urlscanner provider and polls for
// the final result using exponential backoff until success or timeout.
//
// On success, it returns the scan result along with the provider's
// urlscanner.RateLimitStatus. On failure (submission error, polling error that
// never resolves, or context timeout), it returns a non-nil error. In all
// cases, the returned RateLimitStatus reflects the last known rate limit
// information from the provider, allowing callers to adapt scheduling.
//
// Poll timing is controlled by the following constants:
//   - scanResultPollInitialDelay: delay before the first poll attempt
//   - scanResultPollIntervalBase: starting backoff interval
//   - scanResultPollIntervalMax: maximum backoff interval cap
//   - scanResultPollTimeout: overall timeout for the polling operation
func (s scanner) submitURLAndPoll(
	ctx context.Context,
	URL string,
) (*domain.ScanResult, urlscanner.RateLimitStatus, error) {
	logger.Info(ctx, "submitting URL to urlscanner")
	scanRes, RLStatus, err := s.urlScanner.SubmitURL(ctx, URL)
	if err != nil {
		return nil, RLStatus, fmt.Errorf("could not submit URL: %w", err)
	}

	// initial delay
	time.Sleep(scanResultPollInitialDelay)
	// poll for results until timeout
	ctx, cancel := context.WithTimeout(ctx, scanResultPollTimeout)
	defer cancel()
	// start delay with the base interval
	delay := scanResultPollIntervalBase

	for {
		logger.Debug(ctx, "reading results from urlscanner")
		result, err := s.urlScanner.Result(ctx, scanRes.ID)
		if err == nil {
			logger.Debug(ctx, "received results from urlscanner")

			return result, RLStatus, nil
		}

		logger.Debug(ctx, "error reading results from urlscanner, will retry...", zap.Error(err))

		select {
		case <-time.After(delay):
			// double delay each time with a cap
			delay = min(delay*2, scanResultPollIntervalMax)
		case <-ctx.Done():
			return nil, RLStatus, fmt.Errorf("timeout waiting for results: %w", ctx.Err())
		}
	}
}

// New creates a new Scanner instance backed by the provided storage and
// configured with the given options.
func New(storage storage.Storage, URLScanner urlscanner.Client, options Options) Scanner {
	return &scanner{
		options:    options,
		storage:    storage,
		urlScanner: URLScanner,
	}
}
