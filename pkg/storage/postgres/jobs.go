package postgres

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/riverqueue/river"
	"github.com/riverqueue/river/riverdriver/riverdatabasesql"
)

// AddJob enqueues a new River job using the underlying database handle.
//
// Behavior:
//   - If PgSQL is currently operating inside a transaction (DB is a *sql.Tx), the
//     job is inserted using InsertTx so that it participates in the surrounding
//     transaction and will only become visible upon a successful commit.
//   - Otherwise, the job is inserted using a client bound to the *sql.DB, making
//     the operation immediately visible once the insert succeeds.
//
// Any failure to create the River client or to insert the job is returned as an
// error. The provided context controls cancellation and deadlines of the insert
// operation.
func (p *PgSQL) AddJob(ctx context.Context, args river.JobArgs, opts *river.InsertOpts) (bool, error) {
	tx, ok := p.DB.(*sql.Tx)
	if ok {
		riverClient, err := river.NewClient[*sql.Tx](riverdatabasesql.New(nil), &river.Config{})
		if err != nil {
			return false, fmt.Errorf("could not create river queue client: %w", err)
		}

		job, err := riverClient.InsertTx(ctx, tx, args, opts)
		if err != nil {
			return false, fmt.Errorf("could not insert job: %w", err)
		}

		return !job.UniqueSkippedAsDuplicate, nil
	}

	riverClient, err := river.NewClient(riverdatabasesql.New(p.DB.(*sql.DB)), &river.Config{})
	if err != nil {
		return false, fmt.Errorf("could not create river queue client: %w", err)
	}

	job, err := riverClient.Insert(ctx, args, opts)
	if err != nil {
		return false, fmt.Errorf("could not insert job: %w", err)
	}

	return !job.UniqueSkippedAsDuplicate, nil
}
