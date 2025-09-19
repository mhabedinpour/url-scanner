package postgres_test

import (
	"context"
	"database/sql"
	"fmt"
	"path/filepath"
	"scanner/pkg/storage/postgres"
	"testing"
	"time"

	"github.com/pressly/goose/v3"
	"github.com/stretchr/testify/require"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/wait"
)

const (
	testUser     = "postgres"
	testPassword = "postgres"
	testDB       = "testdb"
)

type postgresContainer struct {
	Container testcontainers.Container
	Host      string
	Port      int
}

func startPostgresContainer(ctx context.Context) (*postgresContainer, error) {
	req := testcontainers.ContainerRequest{
		Image:        "postgres:17",
		ExposedPorts: []string{"5432"},
		Env: map[string]string{
			"POSTGRES_USER":     testUser,
			"POSTGRES_PASSWORD": testPassword,
			"POSTGRES_DB":       testDB,
		},
		WaitingFor: wait.ForListeningPort("5432"),
	}
	container, err := testcontainers.GenericContainer(ctx, testcontainers.GenericContainerRequest{
		ContainerRequest: req,
		Started:          true,
	})
	if err != nil {
		return nil, fmt.Errorf("could not start container: %w", err)
	}

	host, err := container.Host(ctx)
	if err != nil {
		return nil, fmt.Errorf("could not get container host: %w", err)
	}

	mappedPort, err := container.MappedPort(ctx, "5432")
	if err != nil {
		return nil, fmt.Errorf("could not get mapped port: %w", err)
	}

	return &postgresContainer{
		Container: container,
		Host:      host,
		Port:      mappedPort.Int(),
	}, nil
}

func runMigrations(db *sql.DB, migrationsDir string) error {
	if err := goose.SetDialect("postgres"); err != nil {
		return fmt.Errorf("could not set dialect: %w", err)
	}

	if err := goose.Up(db, migrationsDir); err != nil {
		return fmt.Errorf("could not run migrations: %w", err)
	}

	return nil
}

func setupTestDB(t *testing.T) (*postgres.PgSQL, func()) {
	t.Helper()
	ctx := context.Background()

	// start container
	pgContainer, err := startPostgresContainer(ctx)
	require.NoError(t, err)

	// create postgres instance
	pgSQL, err := postgres.New(postgres.Options{
		Username:           testUser,
		Password:           testPassword,
		Host:               pgContainer.Host,
		Port:               pgContainer.Port,
		Database:           testDB,
		SslMode:            "disable",
		ConnMaxLifetime:    time.Minute,
		ConnMaxIdleTime:    time.Minute,
		MaxOpenConnections: 5,
		MaxIdleConnections: 5,
	})
	require.NoError(t, err)

	// run migrations
	migrationsDir := filepath.Join("..", "..", "..", "migrations")
	err = runMigrations(pgSQL.DB.(*sql.DB), migrationsDir)
	require.NoError(t, err)

	return pgSQL, func() {
		_ = pgSQL.Close()
		_ = pgContainer.Container.Terminate(ctx)
	}
}
