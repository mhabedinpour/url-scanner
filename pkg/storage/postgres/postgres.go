package postgres

import (
	"context"
	"database/sql"
	"fmt"
	"scanner/pkg/storage"
	"time"

	"github.com/doug-martin/goqu/v9"
	_ "github.com/doug-martin/goqu/v9/dialect/postgres"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/jackc/pgx/v5/stdlib"
)

// Options defines the configuration parameters for PostgreSQL database connection.
type Options struct {
	// Username is the PostgreSQL user to connect as
	Username string
	// Password is the password for the specified user
	Password string
	// Host is the PostgreSQL server hostname or IP address
	Host string
	// SslMode specifies the SSL mode for the connection (e.g., "disable", "require")
	SslMode string
	// Port is the PostgreSQL server port number
	Port int
	// Database is the name of the database to connect to
	Database string
	// ConnMaxLifetime is the maximum amount of time a connection may be reused
	ConnMaxLifetime time.Duration
	// ConnMaxIdleTime is the maximum amount of time a connection may be idle
	ConnMaxIdleTime time.Duration
	// MaxOpenConnections is the maximum number of open connections to the database
	MaxOpenConnections int
	// MaxIdleConnections is the maximum number of connections in the idle connection pool
	MaxIdleConnections int
}

// DB defines the subset of database/sql methods used by this package. Both
// *sql.DB and *sql.Tx satisfy this interface, allowing the same code paths to be
// used within and outside transactions.
type DB interface {
	ExecContext(ctx context.Context, query string, args ...interface{}) (sql.Result, error)
	PrepareContext(ctx context.Context, query string) (*sql.Stmt, error)
	QueryContext(ctx context.Context, query string, args ...interface{}) (*sql.Rows, error)
	QueryRowContext(ctx context.Context, query string, args ...interface{}) *sql.Row
}

// Builder abstracts the minimal subset of goqu methods used by this package to
// construct queries. Both a goqu database handle and a transaction handle
// implement this interface.
type Builder interface {
	From(table ...interface{}) *goqu.SelectDataset
	Insert(table interface{}) *goqu.InsertDataset
	Update(table interface{}) *goqu.UpdateDataset
}

// PgSQL implements the storage.Storage and storage.ScanStorage interfaces for
// PostgreSQL using database/sql and goqu.
type PgSQL struct {
	// DB is the underlying executor. It is either a *sql.DB (when not in a
	// transaction) or a *sql.Tx (when inside a transaction).
	DB DB
	// Builder is the goqu handle used to construct SQL queries bound to DB.
	Builder Builder
	// Pool is the underlying pgx connection Pool used by this storage.
	Pool *pgxpool.Pool
}

// Close closes the underlying pgx connection pool.
func (p *PgSQL) Close() error {
	// Close the pgx Pool if present
	if p.Pool != nil {
		p.Pool.Close()
	}
	// Also close the *sql.DB wrapper if present (best effort)
	if db, ok := p.DB.(*sql.DB); ok {
		_ = db.Close()
	}

	return nil
}

// Commit commits the current transaction. It returns storage.ErrNotInTx if
// called when PgSQL is not in a transactional context.
func (p *PgSQL) Commit() error {
	db, ok := p.DB.(*sql.Tx)
	if !ok {
		return storage.ErrNotInTx
	}

	if err := db.Commit(); err != nil {
		return fmt.Errorf("could not commit tx: %w", err)
	}

	return nil
}

// Rollback aborts the current transaction. It returns storage.ErrNotInTx if
// called when PgSQL is not in a transactional context.
func (p *PgSQL) Rollback() error {
	db, ok := p.DB.(*sql.Tx)
	if !ok {
		return storage.ErrNotInTx
	}

	if err := db.Rollback(); err != nil {
		return fmt.Errorf("could not rollback tx: %w", err)
	}

	return nil
}

// Begin starts a new database transaction and returns a transactional PgSQL
// that can be used to execute subsequent operations within that transaction.
// If called while already inside a transaction, ErrAlreadyInTx is returned.
func (p *PgSQL) Begin(ctx context.Context) (storage.TxStorage, error) {
	db, ok := p.DB.(*sql.DB)
	if !ok {
		return nil, storage.ErrAlreadyInTx
	}

	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return nil, fmt.Errorf("could not begin tx: %w", err)
	}

	return &PgSQL{
		DB:      tx,
		Builder: goqu.NewTx("postgres", tx),
	}, nil
}

// WithTx is a helper that starts a transaction, executes the provided callback
// with a transactional storage handle, and commits if the callback returns nil.
// If the callback returns an error, the transaction is rolled back.
func (p *PgSQL) WithTx(ctx context.Context, cb func(storage storage.AllStorage) error) error {
	tx, err := p.Begin(ctx)
	if err != nil {
		return err
	}

	if err := cb(tx); err != nil {
		_ = tx.Rollback()

		return err
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("could not commit tx: %w", err)
	}

	return nil
}

// New creates a new PostgreSQL storage instance backed by pgxpool, and a
// database/sql wrapper for compatibility with goqu and migrations.
func New(ctx context.Context, options Options) (*PgSQL, error) {
	connStr := fmt.Sprintf("host=%s port=%d user=%s dbname=%s password=%s sslmode=%s",
		options.Host,
		options.Port,
		options.Username,
		options.Database,
		options.Password,
		options.SslMode)
	cfg, err := pgxpool.ParseConfig(connStr)
	if err != nil {
		return nil, fmt.Errorf("could not parse pgxpool config: %w", err)
	}
	if options.MaxOpenConnections > 0 {
		cfg.MaxConns = int32(options.MaxOpenConnections) //nolint: gosec
	}
	if options.MaxIdleConnections > 0 {
		cfg.MinConns = int32(options.MaxIdleConnections) //nolint: gosec
	}
	if options.ConnMaxLifetime > 0 {
		cfg.MaxConnLifetime = options.ConnMaxLifetime
	}
	if options.ConnMaxIdleTime > 0 {
		cfg.MaxConnIdleTime = options.ConnMaxIdleTime
	}

	pool, err := pgxpool.NewWithConfig(ctx, cfg)
	if err != nil {
		return nil, fmt.Errorf("could not create pgx Pool: %w", err)
	}

	// wrap the pool with a *sql.DB to keep compatibility with goqu and goose
	sqlDB := stdlib.OpenDBFromPool(pool)

	return &PgSQL{
		DB:      sqlDB,
		Builder: goqu.Dialect("postgres").DB(sqlDB),
		Pool:    pool,
	}, nil
}
