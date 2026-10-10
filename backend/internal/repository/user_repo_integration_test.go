//go:build integration

package repository

import (
	"context"
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ssr0016/template/internal/database"
	"github.com/ssr0016/template/internal/testutil"
)

func setupTestRepo(t *testing.T) (*UserRepo, *testutil.TestDB) {
	t.Helper()

	tdb := testutil.SetupPostgres(t)
	tdb.RunMigrations(t)
	tdb.TruncateAll(t)

	db := &database.DB{
		Pool:    tdb.Pool,
		Builder: database.NewStatementBuilder(),
	}
	repo := NewUserRepo(db)

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
	assert.Equal(t, 0, user.FailedLoginAttempts)
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

	// Record 3 failed attempts
	for i := 0; i < 3; i++ {
		err := repo.RecordFailedLogin(ctx, user.ID, 5, 15*60*1e9)
		require.NoError(t, err)
	}

	updated, err := repo.GetByID(ctx, user.ID)
	require.NoError(t, err)
	assert.Equal(t, 3, updated.FailedLoginAttempts)
	assert.Nil(t, updated.LockedUntil)
}

func TestUserRepo_ResetLoginAttempts(t *testing.T) {
	repo, _ := setupTestRepo(t)
	ctx := context.Background()

	user, err := repo.CreateWithPassword(ctx, "reset@example.com", "Reset User", "hash")
	require.NoError(t, err)

	// Record some failures
	err = repo.RecordFailedLogin(ctx, user.ID, 5, 15*60*1e9)
	require.NoError(t, err)

	// Reset
	err = repo.ResetLoginAttempts(ctx, user.ID)
	require.NoError(t, err)

	updated, err := repo.GetByID(ctx, user.ID)
	require.NoError(t, err)
	assert.Equal(t, 0, updated.FailedLoginAttempts)
	assert.Nil(t, updated.LockedUntil)
}

func TestUserRepo_ListWithPagination(t *testing.T) {
	repo, _ := setupTestRepo(t)
	ctx := context.Background()

	// Create 5 users
	for i := 1; i <= 5; i++ {
		email := fmt.Sprintf("user%d@example.com", i)
		_, err := repo.CreateWithPassword(ctx, email, "User", "hash")
		require.NoError(t, err)
	}

	// Page 1, limit 2
	users, total, err := repo.ListWithPagination(ctx, "", 1, 2)
	require.NoError(t, err)
	assert.Len(t, users, 2)
	assert.Equal(t, int64(5), total)

	// Page 3, limit 2
	users, _, err = repo.ListWithPagination(ctx, "", 3, 2)
	require.NoError(t, err)
	assert.Len(t, users, 1)
}
