//go:build integration

package repository

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/ssr0016/pastoral-reports/backend/internal/database"
	"github.com/ssr0016/pastoral-reports/backend/internal/testutil"
)

// A verification token is single use: only the first MarkUsed claims it, so
// two requests holding the same token cannot both go on to verify the email.
func TestVerificationRepo_MarkUsedIsSingleUse(t *testing.T) {
	tdb := testutil.SetupPostgres(t)
	tdb.RunMigrations(t)

	db := &database.DB{Pool: tdb.Pool, Builder: database.NewStatementBuilder()}
	users := NewUserRepo(db)
	tokens := NewVerificationRepo(db)
	ctx := context.Background()

	user, err := users.CreateWithPassword(ctx, "verify@example.com", "Verify", "hash")
	require.NoError(t, err)
	token, err := tokens.Create(ctx, user.ID)
	require.NoError(t, err)

	require.NoError(t, tokens.MarkUsed(ctx, token.ID), "first claim wins")
	require.ErrorIs(t, tokens.MarkUsed(ctx, token.ID), ErrNotFound, "second claim must be refused")

	t.Run("missing token is ErrNotFound", func(t *testing.T) {
		require.ErrorIs(t, tokens.MarkUsed(ctx, 99999), ErrNotFound)
	})

	t.Run("used_at is set by the first claim", func(t *testing.T) {
		got, err := tokens.GetByToken(ctx, token.Token)
		require.NoError(t, err)
		require.NotNil(t, got.UsedAt)
	})
}
