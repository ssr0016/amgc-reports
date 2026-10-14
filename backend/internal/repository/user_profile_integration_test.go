//go:build integration

package repository

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ssr0016/pastoral-reports/backend/internal/model"
)

func TestUserRepo_Create_Profile(t *testing.T) {
	repo, tdb := setupTestRepo(t)
	ctx := context.Background()

	var roleID int64
	require.NoError(t, tdb.Pool.QueryRow(ctx,
		`INSERT INTO roles (name, description) VALUES ('pastor-test', 'test') RETURNING id`).Scan(&roleID))

	tests := []struct {
		name     string
		in       model.NewUser
		wantRole func(defaultRole int64) int64
		area     *string
		church   *string
	}{
		{
			name:     "profile and explicit role",
			in:       model.NewUser{Email: "a@example.com", Name: "Ana", PasswordHash: "h", RoleID: roleID, AreaAssignment: "Metro", ChurchName: "AMGC 1"},
			wantRole: func(int64) int64 { return roleID },
			area:     ptr("Metro"), church: ptr("AMGC 1"),
		},
		{
			name:     "no role means the default user role; empty profile is NULL",
			in:       model.NewUser{Email: "b@example.com", Name: "Ben", PasswordHash: "h"},
			wantRole: func(def int64) int64 { return def },
		},
	}
	def, err := repo.CreateWithPassword(ctx, "default@example.com", "Default", "h")
	require.NoError(t, err)

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := repo.Create(ctx, tdb.Pool, tt.in)
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
			list, err := repo.List(ctx, tt.in.Email, 10)
			require.NoError(t, err)
			require.Len(t, list, 1)
			assert.Equal(t, tt.area, list[0].AreaAssignment)
		})
	}
}

func TestUserRepo_Create_Errors(t *testing.T) {
	repo, tdb := setupTestRepo(t)
	ctx := context.Background()
	_, err := repo.Create(ctx, tdb.Pool, model.NewUser{Email: "dup@example.com", Name: "One", PasswordHash: "h"})
	require.NoError(t, err)

	tests := []struct {
		name string
		in   model.NewUser
		want error
	}{
		{"taken email", model.NewUser{Email: "dup@example.com", Name: "Two", PasswordHash: "h"}, ErrUniqueViolation},
		{"unknown role", model.NewUser{Email: "x@example.com", Name: "X", PasswordHash: "h", RoleID: 999999}, ErrForeignKeyViolation},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := repo.Create(ctx, tdb.Pool, tt.in)
			assert.ErrorIs(t, err, tt.want)
		})
	}
}

func TestUserRepo_UpdateProfile(t *testing.T) {
	repo, tdb := setupTestRepo(t)
	ctx := context.Background()
	u, err := repo.Create(ctx, tdb.Pool, model.NewUser{Email: "u@example.com", Name: "Old", PasswordHash: "keep", AreaAssignment: "A", ChurchName: "C"})
	require.NoError(t, err)

	t.Run("replaces name and profile; empty text becomes NULL", func(t *testing.T) {
		got, err := repo.UpdateProfile(ctx, tdb.Pool, u.ID, model.UserProfile{Name: "New", AreaAssignment: "", ChurchName: "C2"})
		require.NoError(t, err)
		assert.Equal(t, "New", got.Name)
		assert.Nil(t, got.AreaAssignment)
		assert.Equal(t, ptr("C2"), got.ChurchName)
		assert.Equal(t, "keep", got.PasswordHash, "the password and role are not touched")
		assert.Equal(t, u.RoleID, got.RoleID)
	})

	t.Run("unknown user", func(t *testing.T) {
		_, err := repo.UpdateProfile(ctx, tdb.Pool, 999999, model.UserProfile{Name: "Nobody"})
		assert.ErrorIs(t, err, ErrNotFound)
	})
}
