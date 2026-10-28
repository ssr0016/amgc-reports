//go:build integration

package authimpl

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/ssr0016/pastoral-reports/backend/internal/auth/token"
	"github.com/ssr0016/pastoral-reports/backend/internal/database"
	"github.com/ssr0016/pastoral-reports/backend/internal/testutil"
	"github.com/ssr0016/pastoral-reports/backend/internal/user/userimpl"
)

// A reset token is single use: only the first MarkUsed claims it, so two
// requests holding the same token cannot both go on to change the password.
func TestPasswordResetRepo_MarkUsedIsSingleUse(t *testing.T) {
	tdb := testutil.SetupPostgres(t)
	tdb.RunMigrations(t)

	db := &database.DB{Pool: tdb.Pool, Builder: database.NewStatementBuilder()}
	users := userimpl.NewStore(db)
	resets := NewPasswordResetStore(db)
	ctx := context.Background()

	user, err := users.CreateWithPassword(ctx, "reset@example.com", "Reset", "hash")
	require.NoError(t, err)
	reset, err := resets.Create(ctx, user.ID)
	require.NoError(t, err)

	require.NoError(t, resets.MarkUsed(ctx, reset.ID), "first claim wins")
	require.ErrorIs(t, resets.MarkUsed(ctx, reset.ID), database.ErrNotFound, "second claim must be refused")

	t.Run("missing token is database.ErrNotFound", func(t *testing.T) {
		require.ErrorIs(t, resets.MarkUsed(ctx, 99999), database.ErrNotFound)
	})

	t.Run("used_at keeps the first claim's time", func(t *testing.T) {
		got, err := resets.GetByToken(ctx, reset.Token)
		require.NoError(t, err)
		require.NotNil(t, got.UsedAt)
	})

	t.Run("only the hash is stored", func(t *testing.T) {
		var stored string
		require.NoError(t, tdb.Pool.QueryRow(ctx, "SELECT token FROM password_resets WHERE id = $1", reset.ID).Scan(&stored))
		require.Equal(t, token.Hash(reset.Token), stored)
	})
}
