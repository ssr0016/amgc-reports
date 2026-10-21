//go:build integration

package authimpl

import (
	"context"
	"log/slog"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"golang.org/x/crypto/bcrypt"

	"github.com/ssr0016/pastoral-reports/backend/internal/auth"
	"github.com/ssr0016/pastoral-reports/backend/internal/database"
	"github.com/ssr0016/pastoral-reports/backend/internal/testutil"
	"github.com/ssr0016/pastoral-reports/backend/internal/user/userimpl"
)

// A deleted user signs in like an unknown email; once restored, an active user signs in again (D-A11).
func TestLogin_DeletedThenRestoredUser(t *testing.T) {
	tdb := testutil.SetupPostgres(t)
	tdb.MigrateWithGoose(t)
	db := &database.DB{Pool: tdb.Pool, Builder: database.NewStatementBuilder()}
	users := userimpl.NewStore(db)
	ctx := context.Background()
	hash, err := bcrypt.GenerateFromPassword([]byte(testPassword), bcrypt.MinCost)
	require.NoError(t, err)
	u, err := users.CreateWithPassword(ctx, "ana@example.com", "Ana", string(hash))
	require.NoError(t, err)
	svc := newService(users, nil, nil, nil, db, slog.New(slog.DiscardHandler))

	_, err = users.SoftDelete(ctx, u.ID, u.ID, "")
	require.NoError(t, err)
	_, err = svc.Login(ctx, "ana@example.com", testPassword)
	assert.ErrorIs(t, err, auth.ErrInvalidCredentials)

	_, err = users.Restore(ctx, u.ID)
	require.NoError(t, err)
	got, err := svc.Login(ctx, "ana@example.com", testPassword)
	require.NoError(t, err)
	assert.Equal(t, u.ID, got.ID)
}
