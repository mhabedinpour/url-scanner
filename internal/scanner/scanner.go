package scanner

import (
	"context"
	"fmt"
	"scanner/internal/config"
	"scanner/pkg/domain"
	"scanner/pkg/serrors"
	"scanner/pkg/storage"
	"time"
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

// New creates a new Scanner instance backed by the provided storage and
// configured with the given options.
func New(storage storage.Storage, options Options) Scanner {
	return &scanner{
		options: options,
		storage: storage,
	}
}
