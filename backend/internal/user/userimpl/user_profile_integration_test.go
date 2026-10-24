//go:build integration

package userimpl

import (
	"context"
	"fmt"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ssr0016/pastoral-reports/backend/internal/database"
	"github.com/ssr0016/pastoral-reports/backend/internal/user"
)

func TestUserRepo_Create_Profile(t *testing.T) {
	repo, tdb := setupTestRepo(t)
	ctx := context.Background()

	var roleID int64
	require.NoError(t, tdb.Pool.QueryRow(ctx,
		`INSERT INTO roles (name, description) VALUES ('pastor-test', 'test') RETURNING id`).Scan(&roleID))

	tests := []struct {
		name     string
		in       user.NewUser
		wantRole func(defaultRole int64) int64
		area     *string
		church   *string
	}{
		{
			name:     "profile and explicit role",
			in:       user.NewUser{Email: "a@example.com", Name: "Ana", PasswordHash: "h", RoleID: roleID, AreaAssignment: "Metro", ChurchName: "AMGC 1"},
			wantRole: func(int64) int64 { return roleID },
			area:     ptr("Metro"), church: ptr("AMGC 1"),
		},
		{
			name:     "no role means the default user role; empty profile is NULL",
			in:       user.NewUser{Email: "b@example.com", Name: "Ben", PasswordHash: "h"},
			wantRole: func(def int64) int64 { return def },
		},
	}
	def, err := repo.CreateWithPassword(ctx, "default@example.com", "Default", "h")
	require.NoError(t, err)

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := repo.Create(ctx, tt.in)
			require.NoError(t, err)
			assert.Equal(t, tt.wantRole(def.RoleID), got.RoleID)
			assert.Equal(t, tt.area, got.AreaAssignment)
			assert.Equal(t, tt.church, got.ChurchName)

			// Every read path returns the profile.
			byID, err := repo.GetByID(ctx, got.ID)
			require.NoError(t, err)
			assert.Equal(t, tt.area, byID.AreaAssignment)
			withRole, err := repo.GetWithRole(ctx, got.ID)
			require.NoError(t, err)
			assert.Equal(t, tt.church, withRole.ChurchName)
			list, err := repo.List(ctx, tt.in.Email, "", 0, 10)
			require.NoError(t, err)
			require.Len(t, list, 1)
			assert.Equal(t, tt.area, list[0].AreaAssignment)
		})
	}
}

func TestUserRepo_Create_Errors(t *testing.T) {
	repo, _ := setupTestRepo(t)
	ctx := context.Background()
	_, err := repo.Create(ctx, user.NewUser{Email: "dup@example.com", Name: "One", PasswordHash: "h"})
	require.NoError(t, err)

	tests := []struct {
		name string
		in   user.NewUser
		want error
	}{
		{"taken email", user.NewUser{Email: "dup@example.com", Name: "Two", PasswordHash: "h"}, database.ErrUniqueViolation},
		{"unknown role", user.NewUser{Email: "x@example.com", Name: "X", PasswordHash: "h", RoleID: 999999}, database.ErrForeignKeyViolation},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := repo.Create(ctx, tt.in)
			assert.ErrorIs(t, err, tt.want)
		})
	}
}

func TestUserRepo_UpdateProfile(t *testing.T) {
	repo, _ := setupTestRepo(t)
	ctx := context.Background()
	u, err := repo.Create(ctx, user.NewUser{Email: "u@example.com", Name: "Old", PasswordHash: "keep", AreaAssignment: "A", ChurchName: "C"})
	require.NoError(t, err)

	t.Run("replaces name and profile; empty text becomes NULL", func(t *testing.T) {
		got, err := repo.UpdateProfile(ctx, u.ID, user.UserProfile{Name: "New", AreaAssignment: "", ChurchName: "C2"})
		require.NoError(t, err)
		assert.Equal(t, "New", got.Name)
		assert.Nil(t, got.AreaAssignment)
		assert.Equal(t, ptr("C2"), got.ChurchName)
		assert.Equal(t, "keep", got.PasswordHash, "the password and role are not touched")
		assert.Equal(t, u.RoleID, got.RoleID)
	})

	t.Run("unknown user", func(t *testing.T) {
		_, err := repo.UpdateProfile(ctx, 999999, user.UserProfile{Name: "Nobody"})
		assert.ErrorIs(t, err, database.ErrNotFound)
	})
}

func ptr(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}

func TestUserRepo_UpdateAvatar(t *testing.T) {
	repo, _ := setupTestRepo(t)
	ctx := context.Background()
	u, err := repo.CreateWithPassword(ctx, "avatar@example.com", "Ava", "h")
	require.NoError(t, err)
	require.Nil(t, u.AvatarURL)

	const url = "https://cdn.example/avatars/1?v=1"
	got, previous, err := repo.UpdateAvatar(ctx, u.ID, url)
	require.NoError(t, err)
	assert.Equal(t, ptr(url), got.AvatarURL)
	assert.Empty(t, previous)

	// GetWithRole feeds /auth/me, so it must load the avatar too.
	withRole, err := repo.GetWithRole(ctx, u.ID)
	require.NoError(t, err)
	assert.Equal(t, ptr(url), withRole.AvatarURL)

	_, _, err = repo.UpdateAvatar(ctx, u.ID+1000, url)
	assert.ErrorIs(t, err, database.ErrNotFound)
}

func TestUserRepo_UpdateAvatar_ConcurrentReplacements(t *testing.T) {
	repo, _ := setupTestRepo(t)
	ctx := context.Background()
	u, err := repo.CreateWithPassword(ctx, "concurrent-avatar@example.com", "Ava", "h")
	require.NoError(t, err)
	const initial = "https://cdn.example/avatars/initial"
	_, _, err = repo.UpdateAvatar(ctx, u.ID, initial)
	require.NoError(t, err)

	type result struct {
		previous string
		err      error
	}
	const uploads = 8
	results := make(chan result, uploads)
	var wg sync.WaitGroup
	for i := range uploads {
		wg.Go(func() {
			_, previous, err := repo.UpdateAvatar(ctx, u.ID, fmt.Sprintf("https://cdn.example/avatars/%d", i))
			results <- result{previous, err}
		})
	}
	wg.Wait()
	close(results)
	previousURLs := make(map[string]bool)
	for result := range results {
		require.NoError(t, result.err)
		require.False(t, previousURLs[result.previous], "each replacement must return a distinct previous URL")
		previousURLs[result.previous] = true
	}
	current, err := repo.GetByID(ctx, u.ID)
	require.NoError(t, err)
	require.NotNil(t, current.AvatarURL)
	assert.True(t, previousURLs[initial])
	assert.False(t, previousURLs[*current.AvatarURL], "the current picture must never be returned for deletion")
	assert.Len(t, previousURLs, uploads)
}
