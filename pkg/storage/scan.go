package storage

import (
	"context"
	"scanner/pkg/domain"
	"time"
)

// ScanUpdates describes a set of optional fields that can be applied to an
// existing scan during an update. Only non-nil fields will be updated.
type ScanUpdates struct {
	// Status is the new status to set for the scan.
	Status domain.ScanStatus
	// Result, when provided, replaces the stored scan result payload.
	Result *domain.ScanResult
	// LastError, when provided, sets the last error text. An empty string value
	// indicates the error should be cleared (set to NULL).
	LastError *string
	// MaxAttempts, when provided alongside a Failed status, ensures that status
	// is only updated to Failed if the current attempts after increment would
	// exceed this threshold. A value <= 0 disables this guard.
	MaxAttempts int
}

// UserScans groups a page of scans returned for a user together with an
// optional NextCursor used for pagination.
type UserScans struct {
	// Scans contains the current page of scan records.
	Scans []domain.Scan
	// NextCursor points to the timestamp to be used as the cursor for fetching
	// the next page. It is nil when there is no next page.
	NextCursor *time.Time
}

// ScanStorage defines CRUD and query operations related to scans. Implementations
// should ensure idempotency and proper handling of soft-deletes where applicable.
type ScanStorage interface {
	// StoreScans inserts one or more scans and returns the stored rows as they
	// exist in the database (including generated fields).
	StoreScans(ctx context.Context, scans ...domain.Scan) ([]domain.Scan, error)
	// UpdatePendingScansByURL updates all pending scans for the given URL using
	// the provided field set.
	// Notes:
	// - Attempts is incremented by 1 and updated_at is set automatically.
	// - If Status is Failed and MaxAttempts > 0, status is only set to Failed
	//   when the attempts after increment would exceed MaxAttempts; otherwise
	//   status remains unchanged (i.e., stays Pending).
	UpdatePendingScansByURL(ctx context.Context, URL string, updates ScanUpdates) error
	// PendingScanCountByURL returns the total number of pending scans for the given URL
	// across all users. Soft-deleted records are excluded from the count.
	PendingScanCountByURL(ctx context.Context, URL string) (int64, error)
	// UpdateScanByID updates a single scan identified by its ID and returns the updated row.
	// The update ignores soft-deleted rows and sets updated_at automatically. Only provided fields are changed.
	UpdateScanByID(ctx context.Context, ID domain.ScanID, updates ScanUpdates) (*domain.Scan, error)
	// DeleteScan performs a soft delete for the given scan ID and user ID and
	// returns the deleted scan, or nil if it was not found.
	DeleteScan(ctx context.Context, userID domain.UserID, ID domain.ScanID) (*domain.Scan, error)
	// UserScans returns a page of scans for a user created before the optional
	// cursor time, limited by the given limit. If status is non-empty, results are
	// filtered to records with the given status.
	UserScans(ctx context.Context,
		userID domain.UserID,
		status domain.ScanStatus,
		cursor time.Time,
		limit uint) (UserScans, error)
	// ScanByID fetches a scan by its ID for the given user, excluding soft-deleted
	// records. Returns nil when not found.
	ScanByID(ctx context.Context, userID domain.UserID, ID domain.ScanID) (*domain.Scan, error)
	// LastCompletedScanByURL returns the most recent completed scan for a given URL across all users.
	// Returns nil when no completed scan exists for the URL.
	LastCompletedScanByURL(ctx context.Context, URL string) (*domain.Scan, error)
}
