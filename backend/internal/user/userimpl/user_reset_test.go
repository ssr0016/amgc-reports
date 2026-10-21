package userimpl

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"golang.org/x/crypto/bcrypt"

	"github.com/ssr0016/pastoral-reports/backend/internal/audit"
	"github.com/ssr0016/pastoral-reports/backend/internal/auth"
	"github.com/ssr0016/pastoral-reports/backend/internal/database"
	"github.com/ssr0016/pastoral-reports/backend/internal/user"
)

const resetPassword = "N3w-Passw0rd-Xv91-mT"

func TestUserService_ResetPassword(t *testing.T) {
	ctx := context.Background()
	newUser := func(f userFixture) *user.User {
		u, err := f.users.Create(ctx, user.NewUser{Email: "u@example.com", Name: "U", PasswordHash: "keep"})
		require.NoError(t, err)
		return u
	}
	// nothingWritten fails the test if the password, the sessions or the lockout are touched.
	nothingWritten := func(t *testing.T, f userFixture) {
		f.users.UpdatePasswordKeepSessionFunc = func(context.Context, int64, string, string) (int64, error) {
			t.Error("the password was written")
			return 0, nil
		}
		f.users.ClearLockoutFunc = func(context.Context, int64) error {
			t.Error("the lockout was cleared")
			return nil
		}
	}

	t.Run("sets the hashed password, ends every session, clears the lockout, one audit row", func(t *testing.T) {
		f := newUserFixture()
		u := newUser(f)
		var gotHash, gotKeep string
		var gotID, clearedID int64
		f.users.UpdatePasswordKeepSessionFunc = func(_ context.Context, id int64, hash, keep string) (int64, error) {
			gotID, gotHash, gotKeep = id, hash, keep
			return 2, nil
		}
		f.users.ClearLockoutFunc = func(_ context.Context, id int64) error {
			clearedID = id
			return nil
		}

		err := f.svc.ResetPassword(ctx, adminID, "10.0.0.2", u.ID, resetPassword)
		require.NoError(t, err)

		assert.Equal(t, u.ID, gotID)
		assert.Empty(t, gotKeep, "an admin reset keeps no session: an empty token ends them all")
		assert.NotEqual(t, resetPassword, gotHash, "the password was stored in the clear")
		require.NoError(t, bcrypt.CompareHashAndPassword([]byte(gotHash), []byte(resetPassword)), "the stored hash must verify the new password")
		assert.Equal(t, u.ID, clearedID)

		entries := f.audit.Entries()
		require.Len(t, entries, 1)
		e := entries[0]
		assert.Equal(t, audit.ActionAdminResetPassword, e.Action)
		assert.Equal(t, audit.TargetTypeUser, e.TargetType)
		assert.Equal(t, fmt.Sprint(u.ID), *e.TargetID)
		assert.Equal(t, "u@example.com", *e.TargetName)
		assert.Equal(t, adminID, *e.PerformedBy)
		assert.Equal(t, "Admin", *e.PerformedByName)
		assert.Equal(t, "admin", *e.PerformedByRole)
		assert.Equal(t, "10.0.0.2", *e.IPAddress)
		assert.Equal(t, map[string]any{"sessions_ended": int64(2)}, e.Details)
		dump := fmt.Sprintf("%+v", e)
		assert.NotContains(t, dump, resetPassword, "the audit row holds the password")
		assert.NotContains(t, dump, gotHash, "the audit row holds the hash")
	})

	t.Run("your own account is refused before anything is hashed or written", func(t *testing.T) {
		f := newUserFixture()
		nothingWritten(t, f)

		err := f.svc.ResetPassword(ctx, adminID, "", adminID, resetPassword)

		assert.ErrorIs(t, err, user.ErrCannotResetOwnPassword)
		assert.Empty(t, f.audit.Entries())
	})

	t.Run("a session whose user is gone is unauthorized", func(t *testing.T) {
		f := newUserFixture()
		u := newUser(f)
		nothingWritten(t, f)

		err := f.svc.ResetPassword(ctx, 12345, "", u.ID, resetPassword)

		assert.ErrorIs(t, err, auth.ErrUnauthorized)
		assert.Empty(t, f.audit.Entries())
	})

	t.Run("a password over 72 bytes is refused, counting bytes and not characters", func(t *testing.T) {
		for name, pw := range map[string]string{
			"73 ASCII bytes":               strings.Repeat("a", 73),
			"37 two-byte characters (74B)": strings.Repeat("é", 37),
		} {
			f := newUserFixture()
			u := newUser(f)
			nothingWritten(t, f)

			err := f.svc.ResetPassword(ctx, adminID, "", u.ID, pw)

			assert.ErrorIs(t, err, auth.ErrPasswordTooLong, name)
			assert.Empty(t, f.audit.Entries(), name)
		}
	})

	t.Run("exactly 72 bytes is accepted", func(t *testing.T) {
		f := newUserFixture()
		u := newUser(f)

		require.NoError(t, f.svc.ResetPassword(ctx, adminID, "", u.ID, strings.Repeat("a", 72)))
	})

	t.Run("an unknown user is not found and nothing is written", func(t *testing.T) {
		f := newUserFixture()
		nothingWritten(t, f)

		err := f.svc.ResetPassword(ctx, adminID, "", 404, resetPassword)

		assert.ErrorIs(t, err, user.ErrUserNotFound)
		assert.Empty(t, f.audit.Entries())
	})

	t.Run("a user deleted between the read and the write is not found", func(t *testing.T) {
		f := newUserFixture()
		u := newUser(f)
		f.users.UpdatePasswordKeepSessionFunc = func(context.Context, int64, string, string) (int64, error) {
			return 0, fmt.Errorf("users: %w", database.ErrNotFound)
		}

		err := f.svc.ResetPassword(ctx, adminID, "", u.ID, resetPassword)

		assert.ErrorIs(t, err, user.ErrUserNotFound)
		assert.Empty(t, f.audit.Entries())
	})

	t.Run("a failed lockout clear fails the reset and writes no audit row", func(t *testing.T) {
		f := newUserFixture()
		u := newUser(f)
		f.users.ClearLockoutFunc = func(context.Context, int64) error { return errors.New("boom") }

		err := f.svc.ResetPassword(ctx, adminID, "", u.ID, resetPassword)

		require.Error(t, err)
		assert.NotErrorIs(t, err, user.ErrUserNotFound)
		assert.NotContains(t, err.Error(), resetPassword)
		assert.Empty(t, f.audit.Entries())
	})

	t.Run("a closed account and another admin can both be reset, and the status is not changed", func(t *testing.T) {
		for _, status := range []string{user.UserStatusInactive, user.UserStatusDeceased} {
			f := newUserFixture()
			u := newUser(f)
			u.Status = status
			f.users.UpdateStatusFunc = func(context.Context, int64, string) (*user.User, error) {
				t.Error("a reset changed the status")
				return nil, nil
			}

			require.NoError(t, f.svc.ResetPassword(ctx, adminID, "", u.ID, resetPassword), status)

			assert.Equal(t, status, u.Status)
		}
	})
}
