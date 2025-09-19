package postgres

import (
	"context"
	"encoding/json"
	"fmt"
	"scanner/pkg/domain"
	"scanner/pkg/storage"
	"time"

	"github.com/doug-martin/goqu/v9"
	"github.com/google/uuid"
)

const (
	scansTable = "scans"
)

func (p *PgSQL) StoreScans(ctx context.Context, scans ...domain.Scan) ([]domain.Scan, error) {
	if len(scans) == 0 {
		return nil, nil
	}

	pgScans, err := domainScansToPg(scans)
	if err != nil {
		return nil, err
	}

	var result []PgScan
	if err := p.Builder.Insert(scansTable).
		Rows(pgScans).
		Returning(&PgScan{}).
		Executor().ScanStructsContext(ctx, &result); err != nil {
		return nil, fmt.Errorf("could not store scans into pg: %w", err)
	}

	return pgScansToDomain(result)
}

// UpdatePendingScansByURL updates all pending scans for the given URL with provided fields.
// Only non-nil fields from updates are set. Attempts is incremented by 1 and updated_at is set.
func (p *PgSQL) UpdatePendingScansByURL(ctx context.Context, URL string, updates storage.ScanUpdates) error {
	rec := goqu.Record{
		"updated_at": goqu.L("CURRENT_TIMESTAMP"),
		"attempts":   goqu.L("attempts + 1"),
		"status":     updates.Status,
	}
	if updates.Result != nil {
		b, err := json.Marshal(updates.Result)
		if err != nil {
			return fmt.Errorf("could not marshal result: %w", err)
		}

		rec["result"] = b
	}
	if updates.LastError != nil {
		if *updates.LastError == "" {
			// set to NULL when empty string provided
			rec["last_error"] = goqu.L("NULL")
		} else {
			rec["last_error"] = *updates.LastError
		}
	}

	_, err := p.Builder.Update(scansTable).
		Set(rec).Where(
		goqu.I("url").Eq(URL),
		goqu.I("status").Eq(string(domain.ScanStatusPending)),
		goqu.I("deleted_at").IsNull(),
	).Executor().ExecContext(ctx)
	if err != nil {
		return fmt.Errorf("could not update pending scans by url in pg: %w", err)
	}

	return nil
}

// DeleteScan performs a soft delete by setting deleted_at timestamp
// for a given scan id and user, returning the deleted record.
func (p *PgSQL) DeleteScan(ctx context.Context, userID domain.UserID, id domain.ScanID) (*domain.Scan, error) {
	var row PgScan
	found, err := p.Builder.Update(scansTable).
		Set(goqu.Record{
			"deleted_at": goqu.L("CURRENT_TIMESTAMP"),
		}).Where(
		goqu.I("id").Eq(uuid.UUID(id)),
		goqu.I("user_id").Eq(uuid.UUID(userID)),
		goqu.I("deleted_at").IsNull(),
	).Returning(&PgScan{}).Executor().ScanStructContext(ctx, &row)
	if err != nil {
		return nil, fmt.Errorf("could not delete scan in pg: %w", err)
	}
	if !found {
		return nil, nil
	}

	return row.ToDomain()
}

// UserScans returns a list of scans for a user filtered by optional cursor and limited by limit.
// Results are ordered by created_at DESC, id DESC. Returns next and previous cursors for pagination.
func (p *PgSQL) UserScans(ctx context.Context,
	userID domain.UserID,
	cursor time.Time,
	limit uint) (storage.UserScans, error) {
	w := []goqu.Expression{
		goqu.I("user_id").Eq(uuid.UUID(userID)),
		goqu.I("deleted_at").IsNull(),
	}
	if !cursor.IsZero() {
		w = append(w, goqu.I("created_at").Lt(cursor))
	}

	// fetch one extra to determine if there is a next page
	fetch := limit + 1
	ds := p.Builder.From(scansTable).
		Where(w...).
		Order(goqu.I("created_at").Desc(), goqu.I("id").Desc()).
		Limit(fetch)

	var rows []PgScan
	if err := ds.Executor().ScanStructsContext(ctx, &rows); err != nil {
		return storage.UserScans{}, fmt.Errorf("could not fetch user scans from pg: %w", err)
	}

	// if we fetched more than the limit, there is a next page
	var nextCursor *time.Time
	if uint(len(rows)) > limit {
		trimmed := rows[:limit]
		nextCursor = &trimmed[len(trimmed)-1].CreatedAt
		rows = trimmed
	}

	domainRows, err := pgScansToDomain(rows)
	if err != nil {
		return storage.UserScans{}, err
	}

	return storage.UserScans{
		Scans:      domainRows,
		NextCursor: nextCursor,
	}, nil
}

// ScanByID returns a scan by its ID, excluding soft-deleted rows.
func (p *PgSQL) ScanByID(ctx context.Context, userID domain.UserID, id domain.ScanID) (*domain.Scan, error) {
	var row PgScan
	found, err := p.Builder.From(scansTable).
		Where(
			goqu.I("id").Eq(uuid.UUID(id)),
			goqu.I("user_id").Eq(uuid.UUID(userID)),
			goqu.I("deleted_at").IsNull(),
		).
		Executor().ScanStructContext(ctx, &row)
	if err != nil {
		return nil, fmt.Errorf("could not fetch scan by id: %w", err)
	}
	if !found {
		return nil, nil
	}

	return row.ToDomain()
}
