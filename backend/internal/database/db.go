package database

import (
	"context"
	"fmt"
	"log/slog"
	"strings"
	"time"

	sq "github.com/Masterminds/squirrel"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// PoolSettings tune the connection pool. A zero field keeps the pgx default.
type PoolSettings struct {
	MaxConns        int32
	MinConns        int32
	MaxConnLifetime time.Duration
	MaxConnIdleTime time.Duration
}

type DB struct {
	Pool    *pgxpool.Pool
	Builder sq.StatementBuilderType
}

// NewWithURL creates a DB using the provided URL and the default pool settings.
func NewWithURL(ctx context.Context, dsn string) (*DB, error) {
	return NewWithPool(ctx, dsn, PoolSettings{})
}

// NewWithPool creates a DB using the provided URL and pool settings.
func NewWithPool(ctx context.Context, dsn string, ps PoolSettings) (*DB, error) {
	if dsn == "" {
		return nil, fmt.Errorf("database URL is required")
	}
	cfg, err := pgxpool.ParseConfig(dsn)
	if err != nil {
		return nil, fmt.Errorf("parse database URL: %w", err)
	}
	applyPool(cfg, ps)
	if PoolerHint(cfg.ConnConfig) {
		slog.Warn("DATABASE_URL uses a pooled endpoint with the default statement cache; "+
			"use the direct endpoint, or add default_query_exec_mode=simple_protocol to the URL",
			"host", cfg.ConnConfig.Host)
	}
	pool, err := pgxpool.NewWithConfig(ctx, cfg)
	if err != nil {
		return nil, fmt.Errorf("pgxpool.New: %w", err)
	}
	if err := pool.Ping(ctx); err != nil {
		return nil, fmt.Errorf("ping: %w", err)
	}
	builder := sq.StatementBuilder.PlaceholderFormat(sq.Dollar)
	return &DB{Pool: pool, Builder: builder}, nil
}

func applyPool(cfg *pgxpool.Config, ps PoolSettings) {
	if ps.MaxConns > 0 {
		cfg.MaxConns = ps.MaxConns
	}
	if ps.MinConns > 0 {
		cfg.MinConns = ps.MinConns
	}
	if ps.MaxConnLifetime > 0 {
		cfg.MaxConnLifetime = ps.MaxConnLifetime
	}
	if ps.MaxConnIdleTime > 0 {
		cfg.MaxConnIdleTime = ps.MaxConnIdleTime
	}
}

// PoolerHint reports whether the DSN points at a pooled endpoint (Neon's "-pooler"
// host runs PgBouncer in transaction mode) while pgx still uses its default
// statement cache. Prepared statements do not survive that pooler, and the
// failures ("prepared statement does not exist") are intermittent. The fix is the
// direct endpoint, or default_query_exec_mode=simple_protocol in the URL.
func PoolerHint(cc *pgx.ConnConfig) bool {
	return strings.Contains(cc.Host, "-pooler") && cc.DefaultQueryExecMode == pgx.QueryExecModeCacheStatement
}

// NewStatementBuilder returns a Squirrel builder configured for Postgres.
func NewStatementBuilder() sq.StatementBuilderType {
	return sq.StatementBuilder.PlaceholderFormat(sq.Dollar)
}
