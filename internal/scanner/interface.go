package scanner

import (
	"context"
	"scanner/pkg/domain"
)

//go:generate mockgen -package mockscanner -source=interface.go -destination=mock/mockscanner.go *
type Scanner interface {
	Enqueue(ctx context.Context, userID domain.UserID, URL string) (*domain.Scan, error)
	UserScans(ctx context.Context,
		userID domain.UserID,
		status domain.ScanStatus,
		cursor string,
		limit uint) ([]domain.Scan, string, error)
	Result(ctx context.Context, userID domain.UserID, scanID domain.ScanID) (*domain.Scan, error)
	Delete(ctx context.Context, userID domain.UserID, scanID domain.ScanID) error
}
