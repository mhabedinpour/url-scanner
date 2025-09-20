package main

import (
	"context"
	"database/sql"
	root "scanner"
	"scanner/internal/config"
	"scanner/pkg/logger"

	"github.com/pressly/goose/v3"
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

			goose.SetBaseFS(root.Migrations)

			if err := goose.SetDialect("postgres"); err != nil {
				logger.Fatal(ctx, "could not set goose dialect to postgres", zap.Error(err))
			}
			if err := goose.Up(strg.DB.(*sql.DB), "migrations"); err != nil {
				logger.Fatal(ctx, "could not migrate pgsql", zap.Error(err))
			}
		},
	}

	return cmd
}
