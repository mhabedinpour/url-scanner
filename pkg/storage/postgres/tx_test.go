package postgres_test

import (
	"context"
	"database/sql"
	"errors"
	"testing"

	"scanner/pkg/storage"
	"scanner/pkg/storage/postgres"

	"github.com/stretchr/testify/require"
)

// createTestTable ensures a dedicated table exists for transactional tests.
func createTestTable(t *testing.T, db *sql.DB) {
	t.Helper()
	_, err := db.ExecContext(context.Background(), `CREATE TABLE IF NOT EXISTS tx_test (
		id SERIAL PRIMARY KEY,
		val INT NOT NULL
	)`)
	require.NoError(t, err)
	_, err = db.ExecContext(context.Background(), `TRUNCATE tx_test`)
	require.NoError(t, err)
}

func countVals(t *testing.T, db *sql.DB, v int) int {
	t.Helper()
	row := db.QueryRowContext(context.Background(), `SELECT COUNT(*) FROM tx_test WHERE val = $1`, v)
	var c int
	require.NoError(t, row.Scan(&c))

	return c
}

func TestPgSQL_Begin_SuccessAndAlreadyInTx(t *testing.T) {
	pg, cleanup := setupTestDB(t)
	defer cleanup()

	createTestTable(t, pg.DB.(*sql.DB))

	ctx := context.Background()

	// Success: begin from *sql.DB
	txStorage, err := pg.Begin(ctx)
	require.NoError(t, err)
	require.NotNil(t, txStorage)

	// Should be a *postgres.PgSQL with underlying *sql.Tx
	inner, ok := txStorage.(*postgres.PgSQL)
	require.True(t, ok)
	_, isTx := inner.DB.(*sql.Tx)
	require.True(t, isTx)

	// Error: begin when already in tx
	_, err = inner.Begin(ctx)
	require.Error(t, err)
	require.ErrorIs(t, err, storage.ErrAlreadyInTx)

	// Cleanup the opened transaction
	require.NoError(t, inner.Rollback())
}

func TestPgSQL_Commit_SuccessAndNotInTx(t *testing.T) {
	pg, cleanup := setupTestDB(t)
	defer cleanup()

	db := pg.DB.(*sql.DB)
	createTestTable(t, db)

	ctx := context.Background()

	// Error path: calling Commit on non-tx
	err := pg.Commit()
	require.Error(t, err)
	require.ErrorIs(t, err, storage.ErrNotInTx)

	// Success path: commit inserts
	txStorage, err := pg.Begin(ctx)
	require.NoError(t, err)
	inner := txStorage.(*postgres.PgSQL)

	_, err = inner.DB.ExecContext(ctx, `INSERT INTO tx_test(val) VALUES ($1)`, 42)
	require.NoError(t, err)

	require.NoError(t, inner.Commit())

	// Verify persistence outside tx
	require.Equal(t, 1, countVals(t, db, 42))
}

func TestPgSQL_Rollback_SuccessAndNotInTx(t *testing.T) {
	pg, cleanup := setupTestDB(t)
	defer cleanup()

	db := pg.DB.(*sql.DB)
	createTestTable(t, db)

	ctx := context.Background()

	// Error path: calling Rollback on non-tx
	err := pg.Rollback()
	require.Error(t, err)
	require.ErrorIs(t, err, storage.ErrNotInTx)

	// Success path: rollback should discard inserts
	txStorage, err := pg.Begin(ctx)
	require.NoError(t, err)
	inner := txStorage.(*postgres.PgSQL)

	_, err = inner.DB.ExecContext(ctx, `INSERT INTO tx_test(val) VALUES ($1)`, 99)
	require.NoError(t, err)

	require.NoError(t, inner.Rollback())

	// Verify no persistence outside tx
	require.Equal(t, 0, countVals(t, db, 99))
}

func TestPgSQL_WithTx_CommitAndRollback(t *testing.T) {
	pg, cleanup := setupTestDB(t)
	defer cleanup()

	db := pg.DB.(*sql.DB)
	createTestTable(t, db)

	ctx := context.Background()

	// Success callback: should commit
	err := pg.WithTx(ctx, func(s storage.AllStorage) error {
		p := s.(*postgres.PgSQL)
		_, e := p.DB.ExecContext(ctx, `INSERT INTO tx_test(val) VALUES ($1)`, 7)

		return e //nolint: wrapcheck
	})
	require.NoError(t, err)
	require.Equal(t, 1, countVals(t, db, 7))

	// Error in callback: should rollback and WithTx returns rollback result (likely nil)
	err = pg.WithTx(ctx, func(s storage.AllStorage) error {
		p := s.(*postgres.PgSQL)
		_, _ = p.DB.ExecContext(ctx, `INSERT INTO tx_test(val) VALUES ($1)`, 9)

		return errors.New("boom")
	})
	require.Error(t, err)
	require.Equal(t, 0, countVals(t, db, 9))
}
