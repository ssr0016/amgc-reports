//go:build integration

package repository

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/ssr0016/pastoral-reports/backend/internal/database"
	"github.com/ssr0016/pastoral-reports/backend/internal/testutil"
)

// A reset token is single use: only the first MarkUsed claims it, so two
// requests holding the same token cannot both go on to change the password.
func TestPasswordResetRepo_MarkUsedIsSingleUse(t *testing.T) {
	tdb := testutil.SetupPostgres(t)
	tdb.RunMigrations(t)

	db := &database.DB{Pool: tdb.Pool, Builder: database.NewStatementBuilder()}
	users := NewUserRepo(db)
	resets := NewPasswordResetRepo(db)
	ctx := context.Background()

	user, err := users.CreateWithPassword(ctx, "reset@example.com", "Reset", "hash")
	require.NoError(t, err)
	token, err := resets.Create(ctx, user.ID)
	require.NoError(t, err)

	require.NoError(t, resets.MarkUsed(ctx, token.ID), "first claim wins")
	require.ErrorIs(t, resets.MarkUsed(ctx, token.ID), ErrNotFound, "second claim must be refused")

	t.Run("missing token is ErrNotFound", func(t *testing.T) {
		require.ErrorIs(t, resets.MarkUsed(ctx, 99999), ErrNotFound)
	})

	t.Run("used_at keeps the first claim's time", func(t *testing.T) {
		got, err := resets.GetByToken(ctx, token.Token)
		require.NoError(t, err)
		require.NotNil(t, got.UsedAt)
	})
}
