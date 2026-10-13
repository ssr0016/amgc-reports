package repository

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

// Querier is the part of a pool or a transaction that repositories need. Both
// *pgxpool.Pool and pgx.Tx satisfy it, so a repository method can run inside a
// transaction opened by WithTx or straight on the pool.
type Querier interface {
	Exec(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error)
	Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error)
	QueryRow(ctx context.Context, sql string, args ...any) pgx.Row
}

var (
	_ Querier = (*pgxpool.Pool)(nil)
	_ Querier = pgx.Tx(nil)
)

// Beginner opens a transaction. *pgxpool.Pool satisfies it.
type Beginner interface {
	Begin(ctx context.Context) (pgx.Tx, error)
}

// WithTx runs fn in one transaction. It commits when fn returns nil, and rolls
// back when fn returns an error or panics (the panic is re-raised).
func WithTx(ctx context.Context, b Beginner, fn func(Querier) error) error {
	tx, err := b.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }() // no-op once committed; covers error and panic

	if err := fn(tx); err != nil {
		return err
	}
	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("commit: %w", err)
	}
	return nil
}
