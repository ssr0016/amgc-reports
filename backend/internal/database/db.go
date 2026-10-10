package database

import (
	"context"
	"fmt"
	"os"

	sq "github.com/Masterminds/squirrel"
	"github.com/jackc/pgx/v5/pgxpool"
)

type DB struct {
	Pool    *pgxpool.Pool
	Builder sq.StatementBuilderType
}

// New creates a DB using DATABASE_URL from environment (backward compatible).
func New(ctx context.Context) (*DB, error) {
	return NewWithURL(ctx, os.Getenv("DATABASE_URL"))
}

// NewWithURL creates a DB using the provided URL.
func NewWithURL(ctx context.Context, dsn string) (*DB, error) {
	if dsn == "" {
		return nil, fmt.Errorf("database URL is required")
	}
	pool, err := pgxpool.New(ctx, dsn)
	if err != nil {
		return nil, fmt.Errorf("pgxpool.New: %w", err)
	}
	if err := pool.Ping(ctx); err != nil {
		return nil, fmt.Errorf("ping: %w", err)
	}
	builder := sq.StatementBuilder.PlaceholderFormat(sq.Dollar)
	return &DB{Pool: pool, Builder: builder}, nil
}

// NewStatementBuilder returns a Squirrel builder configured for Postgres.
func NewStatementBuilder() sq.StatementBuilderType {
	return sq.StatementBuilder.PlaceholderFormat(sq.Dollar)
}
