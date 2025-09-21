package postgres_test

import (
	"context"
	"database/sql"
	"scanner/pkg/storage/postgres"
	"testing"

	"github.com/riverqueue/river"
	"github.com/riverqueue/river/riverdriver/riverdatabasesql"
	"github.com/riverqueue/river/rivermigrate"
	"github.com/riverqueue/river/rivertest"
	"github.com/stretchr/testify/require"
)

type dummyJobArgs struct{}

func (dummyJobArgs) Kind() string { return "dummy" }

func migrateRiver(t *testing.T, storage *postgres.PgSQL) {
	t.Helper()
	migrator, err := rivermigrate.New(riverdatabasesql.New(storage.DB.(*sql.DB)), nil)
	require.NoError(t, err)
	migrations := migrator.AllVersions()
	latestVersion := migrations[len(migrations)-1].Version
	_, err = migrator.Migrate(t.Context(), rivermigrate.DirectionUp, &rivermigrate.MigrateOpts{
		TargetVersion: latestVersion,
	})
	require.NoError(t, err)
}

func TestPgSQL_AddJob_WithinTransaction_UsesTxPath(t *testing.T) {
	pg, cleanup := setupTestDB(t)
	defer cleanup()
	migrateRiver(t, pg)

	ctx := context.Background()

	// Start a transaction to force the *sql.Tx code path in AddJob.
	txStorage, err := pg.Begin(ctx)
	require.NoError(t, err)
	defer func() { _ = txStorage.Rollback() }()

	_, err = txStorage.AddJob(ctx, dummyJobArgs{}, &river.InsertOpts{})
	require.NoError(t, err)
	rivertest.RequireInsertedTx[*riverdatabasesql.Driver](
		ctx,
		t,
		txStorage.(*postgres.PgSQL).DB.(*sql.Tx),
		&dummyJobArgs{},
		nil,
	)
}

func TestPgSQL_AddJob_OutsideTransaction_UsesDBPath(t *testing.T) {
	pg, cleanup := setupTestDB(t)
	defer cleanup()
	migrateRiver(t, pg)

	ctx := context.Background()

	_, err := pg.AddJob(ctx, dummyJobArgs{}, &river.InsertOpts{})
	require.NoError(t, err)
	rivertest.RequireInserted[*riverdatabasesql.Driver](
		ctx,
		t,
		riverdatabasesql.New(pg.DB.(*sql.DB)),
		&dummyJobArgs{},
		nil,
	)
}
