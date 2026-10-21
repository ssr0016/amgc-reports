//go:build integration

package repository

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/ssr0016/pastoral-reports/backend/internal/database"
	"github.com/ssr0016/pastoral-reports/backend/internal/testutil"
)

// txDB is a *database.DB over the test pool.
func txDB(tdb *testutil.TestDB) *database.DB {
	return &database.DB{Pool: tdb.Pool, Builder: database.NewStatementBuilder()}
}

func TestInTransaction_Postgres(t *testing.T) {
	tdb := testutil.SetupPostgres(t)
	db := txDB(tdb)
	ctx := context.Background()
	_, err := tdb.Pool.Exec(ctx, `CREATE TABLE tx_probe (n INT)`)
	require.NoError(t, err)

	count := func() int {
		var n int
		require.NoError(t, tdb.Pool.QueryRow(ctx, `SELECT COUNT(*) FROM tx_probe`).Scan(&n))
		return n
	}
	// insert goes through Querier(ctx), as a repository does.
	insert := func(ctx context.Context) error {
		_, err := db.Querier(ctx).Exec(ctx, `INSERT INTO tx_probe VALUES (1)`)
		return err
	}

	t.Run("nil return persists", func(t *testing.T) {
		require.NoError(t, db.InTransaction(ctx, insert))
		require.Equal(t, 1, count())
	})

	t.Run("error discards the write", func(t *testing.T) {
		errAudit := errors.New("audit write failed")
		err := db.InTransaction(ctx, func(ctx context.Context) error {
			if err := insert(ctx); err != nil {
				return err
			}
			return errAudit
		})
		require.ErrorIs(t, err, errAudit)
		require.Equal(t, 1, count())
	})

	t.Run("panic discards the write", func(t *testing.T) {
		require.Panics(t, func() {
			_ = db.InTransaction(ctx, func(ctx context.Context) error {
				_ = insert(ctx)
				panic("kaboom")
			})
		})
		require.Equal(t, 1, count())
	})

	t.Run("a call without the tx ctx runs outside it", func(t *testing.T) {
		_ = db.InTransaction(ctx, func(context.Context) error {
			require.NoError(t, insert(ctx)) // the outer ctx carries no tx: pool, commits at once
			return errors.New("rollback")
		})
		require.Equal(t, 2, count())
	})

	t.Run("nested call joins the outer transaction", func(t *testing.T) {
		before := count()
		err := db.InTransaction(ctx, func(ctx context.Context) error {
			if err := db.InTransaction(ctx, insert); err != nil {
				return err
			}
			// The inner call did not commit: nothing is visible outside yet.
			require.Equal(t, before, count())
			return errors.New("outer fails")
		})
		require.Error(t, err)
		require.Equal(t, before, count(), "inner work rolled back with the outer transaction")
	})

	t.Run("nested call commits once, with the outer", func(t *testing.T) {
		before := count()
		require.NoError(t, db.InTransaction(ctx, func(ctx context.Context) error {
			return db.InTransaction(ctx, insert)
		}))
		require.Equal(t, before+1, count())
	})
}
