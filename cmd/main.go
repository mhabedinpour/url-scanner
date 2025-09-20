// Package main provides the CLI entrypoint for the URL Scanner service.
// It wires subcommands (scan, migrate, jwt), loads configuration, and initializes logging.
package main

import (
	"context"
	"flag"
	"log"
	"os"
	"scanner/internal/config"
	"scanner/pkg/logger"
	"scanner/pkg/storage/postgres"

	"github.com/spf13/cobra"
	"go.uber.org/zap"
)

// getPostgres creates a PostgreSQL client using configuration values and returns it
// along with a cleanup function to close the connection pool.
func getPostgres(ctx context.Context, cfg *config.Config) (*postgres.PgSQL, func()) {
	pgsql, err := postgres.New(postgres.Options{
		Username:           cfg.Database.Username,
		Password:           cfg.Database.Password,
		Host:               cfg.Database.Host,
		Port:               cfg.Database.Port,
		Database:           cfg.Database.DatabaseName,
		ConnMaxLifetime:    cfg.Database.ConnMaxLifetime,
		ConnMaxIdleTime:    cfg.Database.ConnMaxIdleTime,
		MaxOpenConnections: cfg.Database.MaxOpenConnections,
		MaxIdleConnections: cfg.Database.MaxIdleConnections,
		SslMode:            cfg.Database.SslMode,
	})
	if err != nil {
		logger.Fatal(ctx, "could not create postgres storage", zap.Error(err))
	}

	return pgsql, func() {
		logger.Info(ctx, "closing postgres client...")
		if err = pgsql.Close(); err != nil {
			logger.Warn(ctx, "could not close postgres connection", zap.Error(err))
		}
	}
}

// main sets up the root Cobra command, loads configuration and logging, and
// registers subcommands before executing the CLI.
func main() {
	rootCmd := &cobra.Command{
		Use: "scanner",
	}

	// there is no way to access flags before command execution in cobra.
	// configPath here is parsed using the standard flags package.
	// following line is just added to prevent errors when Cobra is parsing the flags.
	rootCmd.PersistentFlags().StringP("config", "c", "config.yml", "Config File Path")

	configPath := flag.String("c", "config.yml", "The config file path")
	flag.Parse()

	log.Println("loading config ...")
	cfg, err := config.Load(*configPath)
	if err != nil {
		log.Fatal("could not load config file", err)
	}

	logger.Setup(cfg.Environment)

	ctx := context.Background()

	defer func() {
		if p := recover(); p != nil {
			logger.Error(ctx, "captured panic, exiting...", zap.Any("panic", p))
			_ = logger.Get(ctx).Sync()

			panic(p)
		}
	}()

	rootCmd.AddCommand(
		migrateCommand(cfg),
		scanCommand(cfg),
		JWTCommand(cfg),
	)

	err = rootCmd.Execute()
	_ = logger.Get(ctx).Sync()
	if err != nil {
		os.Exit(1) //nolint: gocritic
	}
}
