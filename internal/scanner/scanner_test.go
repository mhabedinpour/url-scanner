package scanner_test

import (
	"context"
	"errors"
	"scanner/internal/scanner"
	mockurlscanner "scanner/pkg/urlscanner/mock"
	"testing"
	"time"

	mockstorage "scanner/pkg/storage/mock"

	"go.uber.org/mock/gomock"

	"scanner/pkg/domain"
	"scanner/pkg/serrors"
	"scanner/pkg/storage"

	"github.com/stretchr/testify/require"
)

const (
	url = "https://example.com/"
)

func newTestScanner(t *testing.T) (*gomock.Controller, *mockstorage.MockStorage, scanner.Scanner) {
	t.Helper()

	ctrl := gomock.NewController(t)
	st := mockstorage.NewMockStorage(ctrl)
	urlScanner := mockurlscanner.NewMockClient(ctrl)
	s := scanner.New(st, urlScanner, scanner.Options{MaxAttempts: 3, ResultCacheTTL: time.Hour})

	return ctrl, st, s
}

// helper to wire Storage.WithTx to execute callback with a MockAllStorage.
func expectWithTx(
	t *testing.T,
	ctrl *gomock.Controller,
	m *mockstorage.MockStorage,
	fn func(tx *mockstorage.MockAllStorage)) {
	t.Helper()

	m.EXPECT().WithTx(gomock.Any(), gomock.Any()).DoAndReturn(
		func(_ context.Context, cb func(storage.AllStorage) error) error {
			// provide a tx mock that implements AllStorage
			tx := mockstorage.NewMockAllStorage(ctrl)
			if fn != nil {
				fn(tx)
			}

			return cb(tx)
		},
	)
}

func TestScanner_Enqueue_JobAdded(t *testing.T) {
	ctrl, st, s := newTestScanner(t)

	userID := domain.UserID{}

	expectWithTx(t, ctrl, st, func(tx *mockstorage.MockAllStorage) {
		// Expect storing the scan
		tx.EXPECT().StoreScans(gomock.Any(), gomock.Any()).DoAndReturn(
			func(_ context.Context, scans ...domain.Scan) ([]domain.Scan, error) {
				// return the same scan with an ID
				ret := scans
				require.Len(t, ret, 1, "expected one scan input")
				ret[0].ID = domain.ScanID{} // zero is fine for test

				return ret, nil
			},
		)
		// Expect adding a job and report it was added
		tx.EXPECT().AddJob(gomock.Any(), gomock.Any(), gomock.Nil()).Return(true, nil)
	})

	scan, err := s.Enqueue(context.Background(), userID, url)
	require.NoError(t, err)
	require.NotNil(t, scan)
	require.Equal(t, url, scan.URL)
	require.Equal(t, domain.ScanStatusPending, scan.Status)
}

func TestScanner_Enqueue_UsesLastCompletedResult(t *testing.T) {
	ctrl, st, s := newTestScanner(t)

	userID := domain.UserID{}
	completed := domain.Scan{Result: domain.ScanResult{}}

	expectWithTx(t, ctrl, st, func(tx *mockstorage.MockAllStorage) {
		tx.EXPECT().StoreScans(gomock.Any(), gomock.Any()).DoAndReturn(
			func(_ context.Context, scans ...domain.Scan) ([]domain.Scan, error) {
				ret := scans
				ret[0].ID = domain.ScanID{}

				return ret, nil
			},
		)
		// Job not added (already exists)
		tx.EXPECT().AddJob(gomock.Any(), gomock.Any(), gomock.Nil()).Return(false, nil)
		// There is a last completed scan for URL
		tx.EXPECT().LastCompletedScanByURL(gomock.Any(), url).Return(&completed, nil)
		// Update the newly created scan to completed with that result
		tx.EXPECT().UpdateScanByID(gomock.Any(), gomock.Any(), gomock.Any()).DoAndReturn(
			func(_ context.Context, _ domain.ScanID, updates storage.ScanUpdates) (*domain.Scan, error) {
				require.Equal(t, domain.ScanStatusCompleted, updates.Status)
				require.NotNil(t, updates.Result, "expected completed update with result")
				res := domain.Scan{Status: domain.ScanStatusCompleted, Result: *updates.Result}

				return &res, nil
			},
		)
	})

	scan, err := s.Enqueue(context.Background(), userID, url)
	require.NoError(t, err)
	require.Equal(t, domain.ScanStatusCompleted, scan.Status)
}

func TestScanner_Enqueue_PendingWhenJobExistsWithoutResult(t *testing.T) {
	ctrl, st, s := newTestScanner(t)
	userID := domain.UserID{}

	expectWithTx(t, ctrl, st, func(tx *mockstorage.MockAllStorage) {
		tx.EXPECT().StoreScans(gomock.Any(), gomock.Any()).DoAndReturn(
			func(_ context.Context, scans ...domain.Scan) ([]domain.Scan, error) {
				ret := scans
				ret[0].ID = domain.ScanID{}

				return ret, nil
			},
		)
		tx.EXPECT().AddJob(gomock.Any(), gomock.Any(), gomock.Nil()).Return(false, nil)
		tx.EXPECT().LastCompletedScanByURL(gomock.Any(), url).Return(nil, nil)
	})

	scan, err := s.Enqueue(context.Background(), userID, url)
	require.NoError(t, err)
	require.Equal(t, domain.ScanStatusPending, scan.Status)
}

func TestScanner_Enqueue_InvalidURL(t *testing.T) {
	_, st, s := newTestScanner(t)
	// No storage calls expected

	_, err := s.Enqueue(context.Background(), domain.UserID{}, "http://[::1")
	require.Error(t, err)
	require.ErrorIs(t, err, serrors.ErrBadRequest)
	// ensure no calls were made on storage
	st.EXPECT().WithTx(gomock.Any(), gomock.Any()).Times(0)
}

func TestScanner_Enqueue_PropagatesErrors(t *testing.T) {
	ctrl, st, s := newTestScanner(t)
	userID := domain.UserID{}

	// error from StoreScans
	expectWithTx(t, ctrl, st, func(tx *mockstorage.MockAllStorage) {
		tx.EXPECT().StoreScans(gomock.Any(), gomock.Any()).Return(nil, errors.New("store err"))
	})
	_, err := s.Enqueue(context.Background(), userID, url)
	require.Error(t, err, "expected error from StoreScans")

	// error from AddJob
	expectWithTx(t, ctrl, st, func(tx *mockstorage.MockAllStorage) {
		tx.EXPECT().StoreScans(gomock.Any(), gomock.Any()).DoAndReturn(
			func(_ context.Context, scans ...domain.Scan) ([]domain.Scan, error) {
				return scans, nil
			},
		)
		tx.EXPECT().AddJob(gomock.Any(), gomock.Any(), gomock.Nil()).Return(false, errors.New("add err"))
	})
	_, err = s.Enqueue(context.Background(), userID, url)
	require.Error(t, err, "expected error from AddJob")

	// error from LastCompletedScanByURL
	expectWithTx(t, ctrl, st, func(tx *mockstorage.MockAllStorage) {
		tx.EXPECT().StoreScans(gomock.Any(), gomock.Any()).DoAndReturn(
			func(_ context.Context, scans ...domain.Scan) ([]domain.Scan, error) { return scans, nil },
		)
		tx.EXPECT().AddJob(gomock.Any(), gomock.Any(), gomock.Nil()).Return(false, nil)
		tx.EXPECT().LastCompletedScanByURL(gomock.Any(), url).Return(nil, errors.New("last err"))
	})
	_, err = s.Enqueue(context.Background(), userID, url)
	require.Error(t, err, "expected error from LastCompletedScanByURL")

	// error from UpdateScanByID
	expectWithTx(t, ctrl, st, func(tx *mockstorage.MockAllStorage) {
		tx.EXPECT().StoreScans(gomock.Any(), gomock.Any()).DoAndReturn(
			func(_ context.Context, scans ...domain.Scan) ([]domain.Scan, error) { return scans, nil },
		)
		tx.EXPECT().AddJob(gomock.Any(), gomock.Any(), gomock.Nil()).Return(false, nil)
		tx.EXPECT().LastCompletedScanByURL(gomock.Any(), url).Return(&domain.Scan{Result: domain.ScanResult{}}, nil)
		tx.EXPECT().UpdateScanByID(gomock.Any(), gomock.Any(), gomock.Any()).Return(nil, errors.New("update err"))
	})
	_, err = s.Enqueue(context.Background(), userID, url)
	require.Error(t, err, "expected error from UpdateScanByID")
}

func TestScanner_UserScans_SuccessAndPagination(t *testing.T) {
	_, st, s := newTestScanner(t)
	userID := domain.UserID{}
	status := domain.ScanStatusPending
	cursorTime := time.Now().Add(-time.Hour).UTC().Truncate(time.Second)
	cursor := cursorTime.Format(time.RFC3339)

	page := storage.UserScans{
		Scans: []domain.Scan{{URL: "https://a"}},
		NextCursor: func() *time.Time {
			t := cursorTime.Add(-time.Minute)

			return &t
		}(),
	}

	st.EXPECT().UserScans(gomock.Any(), userID, status, cursorTime, uint(10)).Return(page, nil)

	scans, next, err := s.UserScans(context.Background(), userID, status, cursor, 10)
	require.NoError(t, err)
	require.Len(t, scans, 1)
	require.Equal(t, "https://a", scans[0].URL)
	require.NotEmpty(t, next, "expected next cursor")
}

func TestScanner_UserScans_InvalidCursor(t *testing.T) {
	_, _, s := newTestScanner(t)
	_, _, err := s.UserScans(context.Background(), domain.UserID{}, "", "not-a-time", 5)
	require.Error(t, err)
	require.ErrorIs(t, err, serrors.ErrBadRequest)
}

func TestScanner_Result(t *testing.T) {
	_, st, s := newTestScanner(t)
	userID := domain.UserID{}
	id := domain.ScanID{}

	// found
	st.EXPECT().ScanByID(gomock.Any(), userID, id).Return(&domain.Scan{URL: "https://x"}, nil)
	scan, err := s.Result(context.Background(), userID, id)
	require.NoError(t, err)
	require.NotNil(t, scan)
	require.Equal(t, "https://x", scan.URL)

	// not found
	st.EXPECT().ScanByID(gomock.Any(), userID, id).Return(nil, nil)
	_, err = s.Result(context.Background(), userID, id)
	require.Error(t, err)
	require.ErrorIs(t, err, serrors.ErrNotFound)

	// storage error
	st.EXPECT().ScanByID(gomock.Any(), userID, id).Return(nil, errors.New("boom"))
	_, err = s.Result(context.Background(), userID, id)
	require.Error(t, err)
}

func TestScanner_Delete(t *testing.T) {
	_, st, s := newTestScanner(t)
	userID := domain.UserID{}
	id := domain.ScanID{}

	// success
	st.EXPECT().DeleteScan(gomock.Any(), userID, id).Return(&domain.Scan{}, nil)
	require.NoError(t, s.Delete(context.Background(), userID, id))
	// not found
	st.EXPECT().DeleteScan(gomock.Any(), userID, id).Return(nil, nil)
	err := s.Delete(context.Background(), userID, id)
	require.Error(t, err)
	require.ErrorIs(t, err, serrors.ErrNotFound)
	// storage error
	st.EXPECT().DeleteScan(gomock.Any(), userID, id).Return(nil, errors.New("boom"))
	require.Error(t, s.Delete(context.Background(), userID, id))
}
