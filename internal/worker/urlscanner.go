package worker

import (
	"context"
	"errors"
	"fmt"
	"scanner/internal/scanner"
	"scanner/pkg/logger"
	"scanner/pkg/serrors"
	"scanner/pkg/urlscanner"
	"sync"
	"time"

	"github.com/riverqueue/river"
	"go.uber.org/zap"
)

// URLScannerWorker is a River worker that scans URLs using a provided scanner.Scanner
// implementation. It embeds River's WorkerDefaults to integrate with the job runtime
// and provides its own cooperative rate limiting. The rate limiting logic ensures that
// we never exceed the upstream API's rate limits while still allowing maximal
// concurrency when budget remains.
//
// # Rate limiting overview
//
// The worker tracks the last known upstream rate-limit status (lastRLStatus) and the
// number of requests currently in flight (inFlightRequests). Before starting a scan,
// reserveRL is called to "reserve" a slot from the current budget. The effective
// remaining budget is computed as:
//
//	remaining := lastRLStatus.Remaining
//	if now > lastRLStatus.ResetAt { remaining = lastRLStatus.Limit }
//
// A request is allowed to start if remaining - inFlightRequests > 0. This allows
// multiple concurrent requests as long as they do not exceed the Remaining budget.
// When there is no budget left, reserveRL waits until either:
//   - the ResetAt time is reached (budget replenishes to Limit), or
//   - another in-flight request finishes and signals requestFinishedChan.
//
// After a request completes, requestFinished is called with the server-provided
// urlscanner.RateLimitStatus gathered from the response. It decrements the
// inFlightRequests counter, notifies any goroutines waiting in reserveRL by sending a
// message on requestFinishedChan (non-blocking), and updates lastRLStatus. The
// update strategy prefers the freshest ResetAt and the lowest Remaining to avoid
// optimistic races when multiple concurrent requests report slightly different views
// of the budget. If ResetAt changes, it is always adopted. Otherwise, Remaining is
// only replaced when it decreases, which is conservative and prevents overuse.
//
// Bootstrap behavior: At startup, before any API call has returned a rate-limit
// status, lastRLStatus is initialized to a synthetic status with Limit=1,
// Remaining=1, and a far-future ResetAt. This permits exactly one request to go
// through so we can obtain real rate-limit headers from the upstream API. Subsequent
// requests use actual data.
//
// Concurrency safety: All rate-limit mutable state is guarded by mu. The
// requestFinishedChan is used as a wake-up signal for waiters without accumulating
// backpressure; send is non-blocking and dropped if no one is waiting.
//
// Error handling: If the scan returns a conflict, the job is canceled. If the scan
// indicates upstream rate limiting, the job is snoozed until ResetAt (deferring
// retry). Other errors are logged and returned.
type URLScannerWorker struct {
	river.WorkerDefaults[scanner.JobArgs]

	// scanner performs the actual URL scan and returns rate-limit status from the
	// upstream API alongside any error.
	scanner scanner.Scanner
	// mu protects all fields below it: inFlightRequests and lastRLStatus.
	mu sync.Mutex
	// inFlightRequests counts how many scans are currently running. It is used in
	// conjunction with lastRLStatus.Remaining to decide if another request may start.
	inFlightRequests int
	// lastRLStatus stores the most recent view of the upstream rate-limit headers.
	// It is updated after each request, preferring newer ResetAt and lower Remaining
	// to avoid optimistic races between concurrent requests.
	lastRLStatus *urlscanner.RateLimitStatus
	// requestFinishedChan is a non-buffered notification channel used to wake up
	// goroutines waiting in reserveRL when any in-flight request completes.
	requestFinishedChan chan struct{}
}

// NewURLScannerWorker constructs a URLScannerWorker using the provided scanner.
// The returned worker enforces cooperative rate limiting across
// its concurrent jobs.
func NewURLScannerWorker(scanner scanner.Scanner) *URLScannerWorker {
	return &URLScannerWorker{
		scanner:             scanner,
		requestFinishedChan: make(chan struct{}),
	}
}

// Work executes a single scan job while respecting rate limits
// It reserves rate-limit budget, runs the scan, updates the
// internal rate-limit state, and maps errors to appropriate River actions.
func (u *URLScannerWorker) Work(ctx context.Context, job *river.Job[scanner.JobArgs]) error {
	ctx = logger.WithFields(ctx, zap.Int64("jobID", job.ID), zap.String("URL", job.Args.URL))

	// try to reserve a rate limit slot
	if err := u.reserveRL(ctx); err != nil {
		logger.Error(ctx, "error reserving rate limit", zap.Error(err))

		return fmt.Errorf("could not reserve rate limit: %w", err)
	}

	RLStatus, err := u.scanner.Scan(ctx, job.Args.URL)
	u.requestFinished(ctx, RLStatus)
	if err != nil {
		if errors.Is(err, serrors.ErrConflict) {
			return river.JobCancel(err) //nolint: wrapcheck
		}

		logger.Error(ctx, "error in scanning URL", zap.Error(err))

		if errors.Is(err, serrors.ErrRateLimited) {
			dur := time.Until(RLStatus.ResetAt)
			if dur < 0 {
				dur = 0
			}

			return river.JobSnooze(dur) //nolint: wrapcheck
		}

		return fmt.Errorf("could not scan URL: %w", err)
	}

	logger.Info(ctx, "URL scanned successfully")

	return nil
}

// requestFinished is called after every scan attempt. It decrements the in-flight
// counter, notifies any goroutines waiting to reserve rate limit, and updates the
// last known rate-limit status using a conservative merge strategy to avoid races
// between concurrent requests.
func (u *URLScannerWorker) requestFinished(ctx context.Context, newRLStatus urlscanner.RateLimitStatus) {
	u.mu.Lock()
	defer u.mu.Unlock()

	if u.inFlightRequests > 0 {
		u.inFlightRequests--
	} else {
		// Defensive clamp: avoid negative values in case of unexpected sequencing.
		u.inFlightRequests = 0
	}

	// If other goroutines are blocked in reserveRL, try to wake exactly one without
	// blocking this goroutine. If no one is waiting, the signal is dropped.
	select {
	case u.requestFinishedChan <- struct{}{}:
	default:
	}

	// If the call didn't return any RL info, don't change our view.
	if newRLStatus.ResetAt.IsZero() {
		return
	}

	log := func() {
		logger.Debug(ctx, "received rate limit status",
			zap.Int("limit", newRLStatus.Limit),
			zap.Int("remaining", newRLStatus.Remaining),
			zap.Time("resetAt", newRLStatus.ResetAt),
			zap.Int("inFlight", u.inFlightRequests))
	}

	// First observation: adopt it unconditionally.
	if u.lastRLStatus == nil {
		u.lastRLStatus = &newRLStatus
		log()

		return
	}

	// If ResetAt changed, always adopt the new window.
	if !u.lastRLStatus.ResetAt.Equal(newRLStatus.ResetAt) {
		u.lastRLStatus = &newRLStatus
		log()

		return
	}

	// Otherwise prefer the lower Remaining to stay conservative under concurrency.
	if newRLStatus.Remaining < u.lastRLStatus.Remaining {
		u.lastRLStatus = &newRLStatus
		log()
	}
}

// reserveRL reserves one unit from the rate-limit budget or blocks until a unit
// becomes available. It implements the cooperative rate limiting described in the
// type-level comment:
//  1. On first use, initialize a synthetic RL state to allow a single probe
//     request to gather real headers.
//  2. Compute effective remaining budget; if we've passed ResetAt, Remaining is
//     treated as Limit.
//  3. If remaining - inFlightRequests > 0, increment inFlightRequests and return.
//  4. Otherwise, wait until either ResetAt elapses or any in-flight request
//     completes (signaled via requestFinishedChan), then retry.
//
// If ctx is canceled while waiting, an error is returned.
func (u *URLScannerWorker) reserveRL(ctx context.Context) error {
	for {
		u.mu.Lock()

		if u.lastRLStatus == nil {
			// At startup allow one request to get feedback from the API.
			u.lastRLStatus = &urlscanner.RateLimitStatus{
				Limit:     1,
				Remaining: 1,
				// Far-future reset so the first reservation doesn't
				// unblock due to a timer; we'll replace this with real headers soon.
				ResetAt: time.Now().Add(365 * 24 * time.Hour),
			}
		}

		remaining := u.lastRLStatus.Remaining
		// If the reset time has passed, treat the full limit as remaining.
		if time.Now().UTC().After(u.lastRLStatus.ResetAt) {
			remaining = u.lastRLStatus.Limit
		}

		// If budget remains once we account for in-flight requests, reserve and go.
		if remaining-u.inFlightRequests > 0 {
			logger.Debug(ctx, "reserved rate limit slot",
				zap.Int("remaining", remaining),
				zap.Int("limit", u.lastRLStatus.Limit),
				zap.Time("resetAt", u.lastRLStatus.ResetAt),
				zap.Int("inFlight", u.inFlightRequests))
			u.inFlightRequests++
			u.mu.Unlock()

			return nil
		}

		// Otherwise, wait for either the reset time (if in the future) or for any
		// request to finish, then retry.
		resetAt := u.lastRLStatus.ResetAt
		u.mu.Unlock()

		logger.Debug(ctx, "waiting for rate limit slot",
			zap.Int("remaining", remaining),
			zap.Int("limit", u.lastRLStatus.Limit),
			zap.Time("resetAt", u.lastRLStatus.ResetAt),
			zap.Int("inFlight", u.inFlightRequests))

		select {
		case <-ctx.Done():
			return fmt.Errorf("timeout waiting for rate limit: %w", ctx.Err())
		case <-u.requestFinishedChan:
			// loop to re-evaluate
			continue
		case <-time.After(time.Until(resetAt)):
			// Reset window elapsed; loop and try again.
			continue
		}
	}
}
