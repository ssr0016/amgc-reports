package main

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"strings"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/joho/godotenv"
	"github.com/pressly/goose/v3"

	"github.com/ssr0016/pastoral-reports/backend/internal/config"
	"github.com/ssr0016/pastoral-reports/backend/internal/database"
	"github.com/ssr0016/pastoral-reports/backend/internal/logger"
)

// migrateConnectTimeout bounds how long `api migrate` waits for the database.
const migrateConnectTimeout = 30 * time.Second

// runMigrate applies the embedded goose migrations and returns, without
// starting the server. It is meant to run as a pre-deploy command, inside the
// platform's network, with the same environment as the server. Every log line
// carries phase=migrate so it is easy to find next to the server's logs.
func runMigrate() error {
	_ = godotenv.Load()

	cfg, err := config.Load()
	if err != nil {
		return err
	}

	log := logger.New(logger.Config{
		Level:  cfg.Logging.Level,
		Format: cfg.Logging.Format,
	}).With("phase", "migrate")
	goose.SetLogger(gooseLogger{log: log})

	ctx, cancel := context.WithTimeout(context.Background(), migrateConnectTimeout)
	defer cancel()

	db, err := database.NewWithURL(ctx, cfg.Database.MigrateURL)
	if err != nil {
		log.Error("cannot connect to the database", "error", err)
		return fmt.Errorf("migrate: %w", err)
	}
	defer db.Pool.Close()

	role, err := currentRole(ctx, db.Pool)
	if err != nil {
		log.Error("cannot read the connected role", "error", err)
		return fmt.Errorf("migrate: %w", err)
	}

	log.Info("applying migrations", "role", role)
	start := time.Now()
	if err := database.RunMigrations(db.Pool); err != nil {
		log.Error("migrations failed", "error", err)
		return fmt.Errorf("migrate: %w", err)
	}
	log.Info("migrations complete", "duration", time.Since(start).Round(time.Millisecond).String())
	return nil
}

// currentRole reports the database role the pool is connected as.
func currentRole(ctx context.Context, pool *pgxpool.Pool) (string, error) {
	var role string
	if err := pool.QueryRow(ctx, `SELECT current_user`).Scan(&role); err != nil {
		return "", fmt.Errorf("read current role: %w", err)
	}
	return role, nil
}

// gooseLogger sends goose's own output (one line per applied migration) through
// the structured logger instead of the standard library's log package.
type gooseLogger struct {
	log *slog.Logger
}

func (l gooseLogger) Printf(format string, v ...any) {
	l.log.Info(strings.TrimSpace(fmt.Sprintf(format, v...)))
}

// Fatalf honours goose's contract that it does not return.
func (l gooseLogger) Fatalf(format string, v ...any) {
	l.log.Error(strings.TrimSpace(fmt.Sprintf(format, v...)))
	os.Exit(1)
}
