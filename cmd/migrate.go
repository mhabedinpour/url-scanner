package main

import (
	"context"
	"database/sql"
	root "scanner"
	"scanner/internal/config"
	"scanner/pkg/logger"

	"github.com/pressly/goose/v3"
	"github.com/riverqueue/river/riverdriver/riverdatabasesql"
	"github.com/riverqueue/river/rivermigrate"
	"github.com/spf13/cobra"
	"go.uber.org/zap"
)

// migrateCommand constructs the 'migrate' subcommand that applies database
// migrations to the latest version using goose.
func migrateCommand(cfg *config.Config) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "migrate",
		Short: "Migrates database to the latest version",
		Run: func(cmd *cobra.Command, args []string) {
			ctx := context.Background()

			strg, closeStrg := getPostgres(ctx, cfg)
			defer closeStrg()

			// goose migrations (internal tables)
			goose.SetBaseFS(root.Migrations)

			if err := goose.SetDialect("postgres"); err != nil {
				logger.Fatal(ctx, "could not set goose dialect to postgres", zap.Error(err))
			}
			if err := goose.Up(strg.DB.(*sql.DB), "migrations"); err != nil {
				logger.Fatal(ctx, "could not migrate pgsql", zap.Error(err))
			}

			// migrate riverqueue
			migrator, err := rivermigrate.New(riverdatabasesql.New(strg.DB.(*sql.DB)), nil)
			if err != nil {
				logger.Fatal(ctx, "could not create river queue migrator", zap.Error(err))
			}
			migrations := migrator.AllVersions()
			latestVersion := migrations[len(migrations)-1].Version
			currentVersion := 0
			currentMigrations, err := migrator.ExistingVersions(ctx)
			if err != nil {
				logger.Fatal(ctx, "could not get existing river queue migrations", zap.Error(err))
			}
			if len(currentMigrations) > 0 {
				currentVersion = currentMigrations[len(currentMigrations)-1].Version
			}
			if latestVersion > currentVersion {
				_, err = migrator.Migrate(ctx, rivermigrate.DirectionUp, &rivermigrate.MigrateOpts{
					TargetVersion: latestVersion,
				})
				if err != nil {
					logger.Fatal(ctx, "could not river queue database", zap.Error(err))
				}
			}
		},
	}

	return cmd
}
