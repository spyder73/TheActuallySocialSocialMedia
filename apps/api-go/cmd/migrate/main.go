package main

import (
	"context"
	"log/slog"
	"os"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/spyder73/TheActuallySocialSocialMedia/apps/api-go/internal/config"
	"github.com/spyder73/TheActuallySocialSocialMedia/apps/api-go/internal/migrate"
)

func main() {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	cfg, err := config.Load()
	if err != nil {
		logger.Error("invalid configuration", "error", err)
		os.Exit(1)
	}
	ctx := context.Background()
	pool, err := pgxpool.New(ctx, cfg.DatabaseURL)
	if err != nil {
		logger.Error("database configuration failed")
		os.Exit(1)
	}
	if err = pool.Ping(ctx); err != nil {
		pool.Close()
		logger.Error("database connection failed")
		os.Exit(1)
	}
	err = migrate.Run(ctx, pool, cfg.MigrationsDir, logger)
	if pool != nil {
		pool.Close()
	}
	if err != nil {
		logger.Error("migration failed", "error", err)
		os.Exit(1)
	}
	logger.Info("migration check complete")
}
