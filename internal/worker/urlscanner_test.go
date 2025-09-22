package worker_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/riverqueue/river"
	"github.com/riverqueue/river/rivertype"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"

	"scanner/internal/scanner"
	mockscanner "scanner/internal/scanner/mock"
	"scanner/internal/worker"
	"scanner/pkg/logger"
	"scanner/pkg/serrors"
	"scanner/pkg/urlscanner"
)

func TestMain(m *testing.M) {
	logger.Setup(logger.DevelopmentEnvironment)
	m.Run()
}

func makeJob(id int64, url string) *river.Job[scanner.JobArgs] {
	return &river.Job[scanner.JobArgs]{
		JobRow: &rivertype.JobRow{ID: id},
		Args:   scanner.JobArgs{URL: url},
	}
}

func TestURLScannerWorker_Work_Success(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mock := mockscanner.NewMockScanner(ctrl)
	w := worker.NewURLScannerWorker(mock)

	// Return some RL status that should be adopted on first success
	rl := urlscanner.RateLimitStatus{Limit: 100, Remaining: 99, ResetAt: time.Now().Add(time.Minute)}
	mock.EXPECT().Scan(gomock.Any(), "https://ok").Return(rl, nil)

	require.NoError(t, w.Work(context.Background(), makeJob(1, "https://ok")))
}

func TestURLScannerWorker_Work_ConflictCancels(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mock := mockscanner.NewMockScanner(ctrl)
	w := worker.NewURLScannerWorker(mock)

	rl := urlscanner.RateLimitStatus{Limit: 100, Remaining: 100, ResetAt: time.Now().Add(time.Minute)}
	mock.EXPECT().Scan(gomock.Any(), "https://conflict").Return(rl, serrors.With(serrors.ErrConflict, "dupe"))

	err := w.Work(context.Background(), makeJob(2, "https://conflict"))
	require.Error(t, err)
	var cancelErr *river.JobCancelError
	require.ErrorAs(t, err, &cancelErr)
}

func TestURLScannerWorker_Work_RateLimitedSnoozes(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mock := mockscanner.NewMockScanner(ctrl)
	w := worker.NewURLScannerWorker(mock)

	resetAt := time.Now().Add(1500 * time.Millisecond)
	rl := urlscanner.RateLimitStatus{Limit: 100, Remaining: 0, ResetAt: resetAt}
	mock.EXPECT().Scan(gomock.Any(), "https://rl").Return(rl, serrors.With(serrors.ErrRateLimited, "provider rl"))

	err := w.Work(context.Background(), makeJob(3, "https://rl"))
	require.Error(t, err)
	var snoozeErr *river.JobSnoozeError
	require.ErrorAs(t, err, &snoozeErr)
	// Duration should be around time.Until(resetAt)
	require.GreaterOrEqual(t, snoozeErr.Duration, 1200*time.Millisecond)
	require.LessOrEqual(t, snoozeErr.Duration, 2*time.Second)
}

func TestURLScannerWorker_Work_GenericErrorWrapped(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mock := mockscanner.NewMockScanner(ctrl)
	w := worker.NewURLScannerWorker(mock)

	rl := urlscanner.RateLimitStatus{Limit: 100, Remaining: 100, ResetAt: time.Now().Add(time.Minute)}
	scanErr := errors.New("boom")
	mock.EXPECT().Scan(gomock.Any(), "https://err").Return(rl, scanErr)

	err := w.Work(context.Background(), makeJob(4, "https://err"))
	require.Error(t, err)
	var cancelErr *river.JobCancelError
	require.NotErrorAs(t, err, &cancelErr, "did not expect JobCancelError")
	var snoozeErr *river.JobSnoozeError
	require.NotErrorAs(t, err, &snoozeErr, "did not expect JobSnoozeError")
}

func TestURLScannerWorker_CooperativeRateLimit_BlocksSecondUntilFirstFinishes(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mock := mockscanner.NewMockScanner(ctrl)
	w := worker.NewURLScannerWorker(mock)

	firstScanStart := make(chan struct{})
	allowFirstToFinish := make(chan struct{})
	secondScanStarted := make(chan struct{})

	// First Scan blocks until we allow it to finish.
	mock.EXPECT().Scan(gomock.Any(), "https://a").
		DoAndReturn(func(ctx context.Context, _ string) (urlscanner.RateLimitStatus, error) {
			close(firstScanStart)
			<-allowFirstToFinish

			return urlscanner.RateLimitStatus{Limit: 1, Remaining: 1, ResetAt: time.Now().Add(time.Minute)}, nil
		})
	// Second Scan should not be called until the first finishes and requestFinished wakes it.
	mock.EXPECT().Scan(gomock.Any(), "https://b").
		DoAndReturn(func(ctx context.Context, _ string) (urlscanner.RateLimitStatus, error) {
			close(secondScanStarted)

			return urlscanner.RateLimitStatus{Limit: 1, Remaining: 1, ResetAt: time.Now().Add(time.Minute)}, nil
		})

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	// Start first work which should proceed immediately.
	go func() { _ = w.Work(ctx, makeJob(10, "https://a")) }()
	// Wait until first Scan has started.
	<-firstScanStart

	// Start second work, which should block before Scan due to RL.
	go func() { _ = w.Work(ctx, makeJob(11, "https://b")) }()

	// Ensure second Scan does NOT start within 100ms while first is still running.
	select {
	case <-secondScanStarted:
		t.Fatal("second scan started before first finished; RL not enforced")
	case <-time.After(100 * time.Millisecond):
		// expected: still blocked
	}

	// Now let the first Scan finish; this should wake the waiter and allow second to start.
	close(allowFirstToFinish)

	select {
	case <-secondScanStarted:
		// success
	case <-time.After(2 * time.Second):
		t.Fatal("second scan did not start after first finished")
	}
}

func TestURLScannerWorker_RL_AllowsUpToRemainingConcurrent_ThenBlocksExtra(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mock := mockscanner.NewMockScanner(ctrl)
	w := worker.NewURLScannerWorker(mock)

	// Prime the worker with RL Remaining=2 so two in-flight can start immediately.
	rlPrime := urlscanner.RateLimitStatus{Limit: 2, Remaining: 2, ResetAt: time.Now().Add(time.Minute)}
	mock.EXPECT().Scan(gomock.Any(), "https://prime").Return(rlPrime, nil)

	require.NoError(t, w.Work(context.Background(), makeJob(20, "https://prime")))

	bStarted := make(chan struct{})
	cStarted := make(chan struct{})
	dStarted := make(chan struct{})
	finishB := make(chan struct{})
	finishC := make(chan struct{})

	// B and C should both be able to start concurrently under Remaining=2.
	mock.EXPECT().Scan(gomock.Any(), "https://b").
		DoAndReturn(func(ctx context.Context, _ string) (urlscanner.RateLimitStatus, error) {
			close(bStarted)
			<-finishB

			// Return Remaining=2 so after B finishes, remaining - inFlight (1) > 0 allowing D to start.
			return urlscanner.RateLimitStatus{Limit: 2, Remaining: 2, ResetAt: time.Now().Add(time.Minute)}, nil
		})
	mock.EXPECT().Scan(gomock.Any(), "https://c").
		DoAndReturn(func(ctx context.Context, _ string) (urlscanner.RateLimitStatus, error) {
			close(cStarted)
			<-finishC

			return urlscanner.RateLimitStatus{Limit: 2, Remaining: 0, ResetAt: time.Now().Add(time.Minute)}, nil
		})
	// D should be blocked until either B or C finishes and wakes a waiter.
	mock.EXPECT().Scan(gomock.Any(), "https://d").
		DoAndReturn(func(ctx context.Context, _ string) (urlscanner.RateLimitStatus, error) {
			close(dStarted)

			return urlscanner.RateLimitStatus{Limit: 2, Remaining: 1, ResetAt: time.Now().Add(time.Minute)}, nil
		})

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	go func() { _ = w.Work(ctx, makeJob(21, "https://b")) }()
	go func() { _ = w.Work(ctx, makeJob(22, "https://c")) }()

	// Wait until both B and C are in-flight.
	select {
	case <-bStarted:
	case <-time.After(time.Second):
		t.Fatal("b did not start in time")
	}
	select {
	case <-cStarted:
	case <-time.After(time.Second):
		t.Fatal("c did not start in time")
	}

	// Start D, which should block before Scan until one finishes.
	go func() { _ = w.Work(ctx, makeJob(23, "https://d")) }()

	select {
	case <-dStarted:
		t.Fatal("d started before any in-flight finished; RL not enforced for Remaining=2")
	case <-time.After(150 * time.Millisecond):
		// expected: still blocked
	}

	// Unblock one (B), which should allow D to start.
	close(finishB)

	select {
	case <-dStarted:
		// success
	case <-time.After(2 * time.Second):
		t.Fatal("d did not start after one request finished")
	}

	// Let C finish to avoid goroutine leaks.
	close(finishC)
}

func TestURLScannerWorker_RL_WaitsForReset_WhenRemainingZero(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mock := mockscanner.NewMockScanner(ctrl)
	w := worker.NewURLScannerWorker(mock)

	// First call returns Remaining=0 with a short ResetAt in the future.
	resetDelay := 300 * time.Millisecond
	resetAt := time.Now().Add(resetDelay)
	rlZero := urlscanner.RateLimitStatus{Limit: 5, Remaining: 0, ResetAt: resetAt}
	mock.EXPECT().Scan(gomock.Any(), "https://a").Return(rlZero, nil)
	require.NoError(t, w.Work(context.Background(), makeJob(30, "https://a")))

	started := make(chan struct{})
	start := time.Now()
	mock.EXPECT().Scan(gomock.Any(), "https://b").
		DoAndReturn(func(ctx context.Context, _ string) (urlscanner.RateLimitStatus, error) {
			close(started)
			// Return any RL status; here we simulate a reset having happened.
			return urlscanner.RateLimitStatus{Limit: 5, Remaining: 4, ResetAt: time.Now().Add(time.Minute)}, nil
		})

	// Start B; it should not invoke Scan until roughly after resetDelay.
	go func() { _ = w.Work(context.Background(), makeJob(31, "https://b")) }()

	select {
	case <-started:
		elapsed := time.Since(start)
		require.GreaterOrEqual(t,
			elapsed,
			resetDelay-75*time.Millisecond,
			"Scan started too early before reset window elapsed")
	case <-time.After(2 * time.Second):
		t.Fatal("b did not start after reset window elapsed")
	}
}

func TestURLScannerWorker_RL_UnblocksOnFailure(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mock := mockscanner.NewMockScanner(ctrl)
	w := worker.NewURLScannerWorker(mock)

	firstStarted := make(chan struct{})
	allowFirstToFinish := make(chan struct{})
	secondStarted := make(chan struct{})

	// First returns a generic error after we allow it to finish.
	mock.EXPECT().Scan(gomock.Any(), "https://fail").
		DoAndReturn(func(ctx context.Context, _ string) (urlscanner.RateLimitStatus, error) {
			close(firstStarted)
			<-allowFirstToFinish

			return urlscanner.RateLimitStatus{Limit: 1, Remaining: 1, ResetAt: time.Now().Add(time.Minute)}, errors.New("boom")
		})
	mock.EXPECT().Scan(gomock.Any(), "https://next").
		DoAndReturn(func(ctx context.Context, _ string) (urlscanner.RateLimitStatus, error) {
			close(secondStarted)

			return urlscanner.RateLimitStatus{Limit: 1, Remaining: 1, ResetAt: time.Now().Add(time.Minute)}, nil
		})

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	go func() { _ = w.Work(ctx, makeJob(40, "https://fail")) }()
	<-firstStarted

	go func() { _ = w.Work(ctx, makeJob(41, "https://next")) }()

	select {
	case <-secondStarted:
		t.Fatal("second started before first failed; RL not enforced")
	case <-time.After(100 * time.Millisecond):
		// expected
	}

	close(allowFirstToFinish)

	select {
	case <-secondStarted:
		// ok
	case <-time.After(2 * time.Second):
		t.Fatal("second did not start after first finished with error")
	}
}
