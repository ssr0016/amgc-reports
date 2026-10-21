package database

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

// Querier is the part of a pool or a transaction that repositories need.
// Both *pgxpool.Pool and pgx.Tx satisfy it.
type Querier interface {
	Exec(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error)
	Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error)
	QueryRow(ctx context.Context, sql string, args ...any) pgx.Row
}

var _ Querier = pgx.Tx(nil)

// Beginner opens a transaction. *pgxpool.Pool satisfies it.
type Beginner interface {
	Begin(ctx context.Context) (pgx.Tx, error)
}

// txKey is the context key under which InTransaction stores the open transaction.
type txKey struct{}

// Querier returns the transaction carried by ctx, or the pool when there is
// none. Every repository query goes through it, so a repository never takes a
// transaction parameter: calling it with the ctx InTransaction passed to fn is
// what puts it in the transaction.
func (db *DB) Querier(ctx context.Context) Querier {
	if tx, ok := ctx.Value(txKey{}).(pgx.Tx); ok {
		return tx
	}
	return db.Pool
}

// InTransaction runs fn in one transaction and passes it a ctx that carries it.
// It commits when fn returns nil, and rolls back when fn returns an error or
// panics (the panic is re-raised).
//
// A call made with a ctx that already carries a transaction joins it: fn runs
// in the outer transaction and nothing is committed or rolled back here, so
// commit and rollback happen once, at the outermost call. An inner error must
// be returned up to the outermost fn, or the outer transaction would commit
// the inner work.
func (db *DB) InTransaction(ctx context.Context, fn func(ctx context.Context) error) error {
	return inTransaction(ctx, db.Pool, fn)
}

func inTransaction(ctx context.Context, b Beginner, fn func(ctx context.Context) error) error {
	if _, ok := ctx.Value(txKey{}).(pgx.Tx); ok {
		return fn(ctx)
	}
	tx, err := b.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }() // no-op once committed; covers error and panic

	if err := fn(context.WithValue(ctx, txKey{}, tx)); err != nil {
		return err
	}
	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("commit: %w", err)
	}
	return nil
}
