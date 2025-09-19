package postgres

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"scanner/pkg/domain"
	"time"

	"github.com/google/uuid"
)

type PgScan struct {
	ID     uuid.UUID `db:"id"      goqu:"skipinsert"`
	UserID uuid.UUID `db:"user_id"`

	URL    string          `db:"url"`
	Status string          `db:"status"`
	Result json.RawMessage `db:"result" goqu:"skipinsert"`

	Attempts  uint           `db:"attempts"   goqu:"skipinsert"`
	LastError sql.NullString `db:"last_error" goqu:"skipinsert"`

	CreatedAt time.Time    `db:"created_at" goqu:"skipinsert"`
	UpdatedAt sql.NullTime `db:"updated_at" goqu:"skipinsert"`
	DeletedAt sql.NullTime `db:"deleted_at" goqu:"skipinsert"`
}

// TODO: use https://github.com/jmattheis/goverter for converting

func (p *PgScan) ToDomain() (*domain.Scan, error) {
	var result domain.ScanResult
	if err := json.Unmarshal(p.Result, &result); err != nil {
		return nil, fmt.Errorf("could not unmarshal scan result: %w", err)
	}

	return &domain.Scan{
		ID:        domain.ScanID(p.ID),
		UserID:    domain.UserID(p.UserID),
		URL:       p.URL,
		Status:    domain.ScanStatus(p.Status),
		Result:    result,
		Attempts:  p.Attempts,
		LastError: p.LastError.String,
		CreatedAt: p.CreatedAt,
		UpdatedAt: p.UpdatedAt.Time,
		DeletedAt: p.DeletedAt.Time,
	}, nil
}

func (p *PgScan) FromDomain(scan domain.Scan) error {
	result, err := json.Marshal(scan.Result)
	if err != nil {
		return fmt.Errorf("could not marshal scan result: %w", err)
	}

	*p = PgScan{
		ID:       uuid.UUID(scan.ID),
		UserID:   uuid.UUID(scan.UserID),
		URL:      scan.URL,
		Status:   string(scan.Status),
		Result:   result,
		Attempts: scan.Attempts,
		LastError: sql.NullString{
			String: scan.LastError,
			Valid:  scan.LastError != "",
		},
		CreatedAt: scan.CreatedAt,
		UpdatedAt: sql.NullTime{
			Time:  scan.UpdatedAt,
			Valid: !scan.UpdatedAt.IsZero(),
		},
		DeletedAt: sql.NullTime{
			Time:  scan.DeletedAt,
			Valid: !scan.DeletedAt.IsZero(),
		},
	}

	return nil
}

func domainScansToPg(scans []domain.Scan) ([]PgScan, error) {
	out := make([]PgScan, len(scans))
	for i := range out {
		if err := out[i].FromDomain(scans[i]); err != nil {
			return nil, err
		}
	}

	return out, nil
}

func pgScansToDomain(scans []PgScan) ([]domain.Scan, error) {
	out := make([]domain.Scan, 0, len(scans))
	for _, scan := range scans {
		d, err := scan.ToDomain()
		if err != nil {
			return nil, err
		}

		out = append(out, *d)
	}

	return out, nil
}
