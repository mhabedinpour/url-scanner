package scanner

import (
	"context"
	"fmt"
	"scanner/pkg/domain"
	"scanner/pkg/serrors"
	"scanner/pkg/storage"
	"time"
)

type scanner struct {
	storage storage.Storage
}

func (s scanner) Enqueue(ctx context.Context, userID domain.UserID, URL string) (*domain.Scan, error) {
	res, err := s.storage.StoreScans(ctx, domain.Scan{
		UserID: userID,
		URL:    URL,
		Status: domain.ScanStatusPending,
	})
	if err != nil {
		return nil, fmt.Errorf("could not enqueue URL: %w", err)
	}

	return &res[0], nil
}

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

func (s scanner) Delete(ctx context.Context, userID domain.UserID, scanID domain.ScanID) error {
	res, err := s.storage.DeleteScan(ctx, userID, scanID)
	if err != nil {
		return fmt.Errorf("could not delete scan: %w", err)
	}
	if res == nil {
		return serrors.With(serrors.ErrNotFound, "scan not found")
	}

	return nil
}

func New(storage storage.Storage) Scanner {
	return &scanner{
		storage: storage,
	}
}
