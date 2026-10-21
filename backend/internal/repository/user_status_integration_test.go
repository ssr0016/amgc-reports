//go:build integration

package repository

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ssr0016/pastoral-reports/backend/internal/model"
)

func mustAdmin(t *testing.T, users *UserRepo, email string, roleID int64) int64 {
	t.Helper()
	u, err := users.Create(context.Background(), model.NewUser{Email: email, Name: "Admin", PasswordHash: "h", RoleID: roleID})
	require.NoError(t, err)
	return u.ID
}

func TestUserRepo_UpdateStatus(t *testing.T) {
	users, _ := setupSessions(t)
	ctx := context.Background()
	u, err := users.CreateWithPassword(ctx, "a@example.com", "A", "hash")
	require.NoError(t, err)
	assert.Equal(t, "active", u.Status, "a new account is active")

	got, err := users.UpdateStatus(ctx, u.ID, "inactive")

	require.NoError(t, err)
	assert.Equal(t, "inactive", got.Status)
	again, err := users.GetByID(ctx, u.ID)
	require.NoError(t, err)
	assert.Equal(t, "inactive", again.Status)
	byEmail, err := users.GetByEmail(ctx, "a@example.com")
	require.NoError(t, err)
	assert.Equal(t, "inactive", byEmail.Status)
	withRole, err := users.GetWithRole(ctx, u.ID)
	require.NoError(t, err)
	assert.Equal(t, "inactive", withRole.Status)
}

func TestUserRepo_UpdateStatus_MissingUser(t *testing.T) {
	users, _ := setupSessions(t)

	_, err := users.UpdateStatus(context.Background(), 99999, "inactive")

	require.ErrorIs(t, err, ErrNotFound)
}

func TestUserRepo_RevokeSessions_EndsOnlyThatUsersLiveSessions(t *testing.T) {
	users, tdb := setupSessions(t)
	ctx := context.Background()
	a, err := users.CreateWithPassword(ctx, "a@example.com", "A", "hash")
	require.NoError(t, err)
	b, err := users.CreateWithPassword(ctx, "b@example.com", "B", "hash")
	require.NoError(t, err)
	addSession(t, tdb, "a1", a.ID)
	addSession(t, tdb, "a2", a.ID)
	addSession(t, tdb, "b1", b.ID)

	n, err := users.RevokeSessions(ctx, a.ID)

	require.NoError(t, err)
	assert.EqualValues(t, 2, n)
	assert.Zero(t, liveSessions(t, tdb, a.ID))
	assert.Equal(t, 1, liveSessions(t, tdb, b.ID), "another user's session was revoked")
}

func TestUserRepo_LockActiveAdmins_ReturnsOnlyActiveAdmins(t *testing.T) {
	users, tdb := setupSessions(t)
	ctx := context.Background()
	var adminRole int64
	require.NoError(t, tdb.Pool.QueryRow(ctx,
		`INSERT INTO roles (name, description) VALUES ('admin', 'test') RETURNING id`).Scan(&adminRole))
	plain, err := users.CreateWithPassword(ctx, "plain@example.com", "Plain", "hash")
	require.NoError(t, err)
	a1 := mustAdmin(t, users, "a1@example.com", adminRole)
	a2 := mustAdmin(t, users, "a2@example.com", adminRole)
	gone := mustAdmin(t, users, "gone@example.com", adminRole)
	_, err = users.UpdateStatus(ctx, gone, "inactive")
	require.NoError(t, err)

	ids, err := users.LockActiveAdmins(ctx)

	require.NoError(t, err)
	assert.Equal(t, []int64{a1, a2}, ids, "only active admins, in id order")
	assert.NotContains(t, ids, plain.ID)
}

func TestUserRepo_List_FiltersByStatus(t *testing.T) {
	users, _ := setupSessions(t)
	ctx := context.Background()
	mk := func(email, status string) {
		u, err := users.CreateWithPassword(ctx, email, "N", "h")
		require.NoError(t, err)
		_, err = users.UpdateStatus(ctx, u.ID, status)
		require.NoError(t, err)
	}
	mk("a@example.com", "active")
	mk("b@example.com", "inactive")
	mk("c@example.com", "deceased")
	emails := func(list []model.User, err error) []string {
		require.NoError(t, err)
		var out []string
		for _, u := range list {
			out = append(out, u.Email)
		}
		return out
	}

	assert.ElementsMatch(t, []string{"a@example.com", "b@example.com", "c@example.com"}, emails(users.List(ctx, "", "", 100)), "no status means everyone")
	assert.Equal(t, []string{"b@example.com"}, emails(users.List(ctx, "", "inactive", 100)))
	assert.Equal(t, []string{"c@example.com"}, emails(users.List(ctx, "c@", "deceased", 100)), "email and status combine")
	assert.Empty(t, emails(users.List(ctx, "a@", "deceased", 100)))
}
