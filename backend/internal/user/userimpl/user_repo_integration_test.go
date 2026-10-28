//go:build integration

package userimpl

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ssr0016/pastoral-reports/backend/internal/database"
	"github.com/ssr0016/pastoral-reports/backend/internal/testutil"
	"github.com/ssr0016/pastoral-reports/backend/internal/user"
	"github.com/ssr0016/pastoral-reports/backend/pkg/pagination"
)

func setupTestRepo(t *testing.T) (user.Store, *testutil.TestDB) {
	t.Helper()

	tdb := testutil.SetupPostgres(t)
	tdb.RunMigrations(t)
	tdb.TruncateAll(t)

	db := &database.DB{
		Pool:    tdb.Pool,
		Builder: database.NewStatementBuilder(),
	}
	repo := NewStore(db)

	return repo, tdb
}

func TestUserRepo_CreateWithPassword(t *testing.T) {
	repo, _ := setupTestRepo(t)
	ctx := context.Background()

	user, err := repo.CreateWithPassword(ctx, "test@example.com", "Test User", "hash123")
	require.NoError(t, err)
	require.NotNil(t, user)

	assert.NotZero(t, user.ID)
	assert.Equal(t, "test@example.com", user.Email)
	assert.Equal(t, "Test User", user.Name)
	assert.Equal(t, "hash123", user.PasswordHash)
	assert.NotZero(t, user.RoleID) // Default 'user' role
	assert.False(t, user.EmailVerified)
}

func TestUserRepo_GetByID(t *testing.T) {
	repo, _ := setupTestRepo(t)
	ctx := context.Background()

	created, err := repo.CreateWithPassword(ctx, "get@example.com", "Get User", "hash")
	require.NoError(t, err)

	user, err := repo.GetByID(ctx, created.ID)
	require.NoError(t, err)
	require.NotNil(t, user)

	assert.Equal(t, created.ID, user.ID)
	assert.Equal(t, "get@example.com", user.Email)
}

func TestUserRepo_GetByID_NotFound(t *testing.T) {
	repo, _ := setupTestRepo(t)
	ctx := context.Background()

	user, err := repo.GetByID(ctx, 99999)
	require.NoError(t, err)
	assert.Nil(t, user)
}

func TestUserRepo_GetByEmail(t *testing.T) {
	repo, _ := setupTestRepo(t)
	ctx := context.Background()

	_, err := repo.CreateWithPassword(ctx, "email@example.com", "Email User", "hash")
	require.NoError(t, err)

	user, err := repo.GetByEmail(ctx, "email@example.com")
	require.NoError(t, err)
	require.NotNil(t, user)
	assert.Equal(t, "email@example.com", user.Email)
}

func TestUserRepo_GetByEmail_NotFound(t *testing.T) {
	repo, _ := setupTestRepo(t)
	ctx := context.Background()

	user, err := repo.GetByEmail(ctx, "nobody@example.com")
	require.NoError(t, err)
	assert.Nil(t, user)
}

func TestUserRepo_MarkEmailVerified(t *testing.T) {
	repo, _ := setupTestRepo(t)
	ctx := context.Background()

	user, err := repo.CreateWithPassword(ctx, "verify@example.com", "Verify User", "hash")
	require.NoError(t, err)
	assert.False(t, user.EmailVerified)

	err = repo.MarkEmailVerified(ctx, user.ID)
	require.NoError(t, err)

	updated, err := repo.GetByID(ctx, user.ID)
	require.NoError(t, err)
	assert.True(t, updated.EmailVerified)
	assert.NotNil(t, updated.EmailVerifiedAt)
}

func TestUserRepo_UpdatePassword(t *testing.T) {
	repo, _ := setupTestRepo(t)
	ctx := context.Background()

	user, err := repo.CreateWithPassword(ctx, "update@example.com", "Update User", "oldhash")
	require.NoError(t, err)

	err = repo.UpdatePassword(ctx, user.ID, "newhash")
	require.NoError(t, err)

	updated, err := repo.GetByID(ctx, user.ID)
	require.NoError(t, err)
	assert.Equal(t, "newhash", updated.PasswordHash)
}

func TestUserRepo_RecordFailedLogin(t *testing.T) {
	repo, _ := setupTestRepo(t)
	ctx := context.Background()

	user, err := repo.CreateWithPassword(ctx, "lock@example.com", "Lock User", "hash")
	require.NoError(t, err)

	for range 2 {
		require.NoError(t, repo.RecordFailedLogin(ctx, user.ID, "1.1.1.1", 3, time.Hour))
	}
	locked, err := repo.LoginLocked(ctx, user.ID, "1.1.1.1")
	require.NoError(t, err)
	assert.False(t, locked, "locked below the threshold")

	require.NoError(t, repo.RecordFailedLogin(ctx, user.ID, "1.1.1.1", 3, time.Hour))
	locked, err = repo.LoginLocked(ctx, user.ID, "1.1.1.1")
	require.NoError(t, err)
	assert.True(t, locked, "not locked at the threshold")

	// M2: the lock is per IP, so another IP is not locked out.
	locked, err = repo.LoginLocked(ctx, user.ID, "2.2.2.2")
	require.NoError(t, err)
	assert.False(t, locked, "another IP was locked out")
}

func TestUserRepo_ResetLoginAttempts(t *testing.T) {
	repo, _ := setupTestRepo(t)
	ctx := context.Background()

	user, err := repo.CreateWithPassword(ctx, "reset@example.com", "Reset User", "hash")
	require.NoError(t, err)

	for _, ip := range []string{"1.1.1.1", "2.2.2.2"} {
		require.NoError(t, repo.RecordFailedLogin(ctx, user.ID, ip, 1, time.Hour))
	}

	require.NoError(t, repo.ResetLoginAttempts(ctx, user.ID, "1.1.1.1"))

	locked, err := repo.LoginLocked(ctx, user.ID, "1.1.1.1")
	require.NoError(t, err)
	assert.False(t, locked)
	locked, err = repo.LoginLocked(ctx, user.ID, "2.2.2.2")
	require.NoError(t, err)
	assert.True(t, locked, "a right password from one IP cleared another IP's lock")
}

func TestUserRepo_ListPages(t *testing.T) {
	repo, _ := setupTestRepo(t)
	ctx := context.Background()

	// Create 5 users
	for i := 1; i <= 5; i++ {
		email := fmt.Sprintf("user%d@example.com", i)
		_, err := repo.CreateWithPassword(ctx, email, "User", "hash")
		require.NoError(t, err)
	}

	// Page 1, limit 2
	users, total, err := repo.List(ctx, "", "", 0, pagination.Params{Page: 1, Limit: 2})
	require.NoError(t, err)
	assert.Len(t, users, 2)
	assert.Equal(t, int64(5), total)

	// Page 3, limit 2
	users, total, err = repo.List(ctx, "", "", 0, pagination.Params{Page: 3, Limit: 2})
	require.NoError(t, err)
	assert.Len(t, users, 1)
	assert.Equal(t, int64(5), total, "the total counts every match, not the page")
}
