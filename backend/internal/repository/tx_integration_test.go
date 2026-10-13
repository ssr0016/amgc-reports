//go:build integration

package repository

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/ssr0016/pastoral-reports/backend/internal/testutil"
)

func TestWithTx_Postgres(t *testing.T) {
	tdb := testutil.SetupPostgres(t)
	ctx := context.Background()
	_, err := tdb.Pool.Exec(ctx, `CREATE TABLE tx_probe (n INT)`)
	require.NoError(t, err)

	count := func() int {
		var n int
		require.NoError(t, tdb.Pool.QueryRow(ctx, `SELECT COUNT(*) FROM tx_probe`).Scan(&n))
		return n
	}
	insert := func(q Querier) error {
		_, err := q.Exec(ctx, `INSERT INTO tx_probe VALUES (1)`)
		return err
	}

	t.Run("nil return persists", func(t *testing.T) {
		require.NoError(t, WithTx(ctx, tdb.Pool, insert))
		require.Equal(t, 1, count())
	})

	t.Run("error discards the write", func(t *testing.T) {
		errAudit := errors.New("audit write failed")
		err := WithTx(ctx, tdb.Pool, func(q Querier) error {
			if err := insert(q); err != nil {
				return err
			}
			return errAudit
		})
		require.ErrorIs(t, err, errAudit)
		require.Equal(t, 1, count())
	})

	t.Run("panic discards the write", func(t *testing.T) {
		require.Panics(t, func() {
			_ = WithTx(ctx, tdb.Pool, func(q Querier) error {
				_ = insert(q)
				panic("kaboom")
			})
		})
		require.Equal(t, 1, count())
	})
}
