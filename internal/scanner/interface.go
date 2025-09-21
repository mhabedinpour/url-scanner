// Package scanner provides the core interfaces and implementations for scheduling and retrieving URL scans.
package scanner

import (
	"context"
	"scanner/pkg/domain"
)

// Scanner is the main interface for scheduling URL scans and querying their results.
// Implementations are expected to enqueue scan jobs, paginate user scans,
// fetch individual scan results, and delete scans when requested.
//
//go:generate mockgen -package mockscanner -source=interface.go -destination=mock/mockscanner.go *
type Scanner interface {
	// Enqueue submits a new scan request for the given URL and user.
	// It returns the created scan record, which may already be completed if a
	// recent cached result exists for the same URL.
	Enqueue(ctx context.Context, userID domain.UserID, URL string) (*domain.Scan, error)

	// UserScans returns a page of scans for the given user filtered by status.
	// Cursor is an RFC3339 timestamp string; when empty, it starts from "now".
	// The returned string is the next cursor to request the following page.
	UserScans(ctx context.Context,
		userID domain.UserID,
		status domain.ScanStatus,
		cursor string,
		limit uint) ([]domain.Scan, string, error)

	// Result fetches a single scan by ID for the given user, or a not-found error
	// when the scan does not exist.
	Result(ctx context.Context, userID domain.UserID, scanID domain.ScanID) (*domain.Scan, error)

	// Delete removes a scan belonging to the given user. If the scan does not
	// exist, a not-found error is returned.
	Delete(ctx context.Context, userID domain.UserID, scanID domain.ScanID) error
}
