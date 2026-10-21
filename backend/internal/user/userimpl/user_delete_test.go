package userimpl

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ssr0016/pastoral-reports/backend/internal/audit"
	"github.com/ssr0016/pastoral-reports/backend/internal/user"
)

func TestUserService_Delete(t *testing.T) {
	ctx := context.Background()
	newUser := func(f userFixture) *user.User {
		u, err := f.users.Create(ctx, user.NewUser{Email: "u@example.com", Name: "U", PasswordHash: "keep"})
		require.NoError(t, err)
		return u
	}

	t.Run("marks the user deleted, ends sessions, one audit row; the status stays", func(t *testing.T) {
		f := newUserFixture()
		u := newUser(f)
		f.users.RevokeSessionsFunc = func(context.Context, int64) (int64, error) { return 2, nil }

		got, err := f.svc.Delete(ctx, adminID, "10.0.0.2", u.ID, "  left the church  ")
		require.NoError(t, err)
		require.NotNil(t, got.DeletedAt)
		assert.Equal(t, user.UserStatusActive, got.Status)
		require.NotNil(t, got.DeletedBy)
		assert.Equal(t, adminID, *got.DeletedBy)

		entries := f.audit.Entries()
		require.Len(t, entries, 1)
		assert.Equal(t, audit.ActionDeleteUser, entries[0].Action)
		assert.Equal(t, "u@example.com", *entries[0].TargetName)
		assert.Equal(t, adminID, *entries[0].PerformedBy)
		assert.Equal(t, map[string]any{"reason": "left the church", "sessions_revoked": int64(2)}, entries[0].Details)
	})

	t.Run("a blank reason is audited as null and stored as NULL", func(t *testing.T) {
		f := newUserFixture()
		u := newUser(f)
		got, err := f.svc.Delete(ctx, adminID, "", u.ID, "   ")
		require.NoError(t, err)
		assert.Nil(t, got.DeletedReason)
		assert.Equal(t, map[string]any{"reason": nil, "sessions_revoked": int64(0)}, f.audit.Entries()[0].Details)
	})

	t.Run("yourself is ErrCannotDeleteSelf before any write", func(t *testing.T) {
		f := newUserFixture()
		f.users.LockActiveAdminsFunc = func(context.Context) ([]int64, error) {
			t.Error("the transaction started for a self-delete")
			return nil, nil
		}
		_, err := f.svc.Delete(ctx, adminID, "", adminID, "")
		assert.ErrorIs(t, err, user.ErrCannotDeleteSelf)
		assert.Empty(t, f.audit.Entries())
	})

	t.Run("the only active admin is ErrLastActiveAdmin and nothing changes", func(t *testing.T) {
		f := newUserFixture()
		u := newUser(f)
		f.users.LockActiveAdminsFunc = func(context.Context) ([]int64, error) { return []int64{u.ID}, nil }
		f.users.RevokeSessionsFunc = func(context.Context, int64) (int64, error) {
			t.Error("sessions revoked for a refused delete")
			return 0, nil
		}
		_, err := f.svc.Delete(ctx, adminID, "", u.ID, "")
		assert.ErrorIs(t, err, user.ErrLastActiveAdmin)
		assert.Nil(t, u.DeletedAt)
		assert.Empty(t, f.audit.Entries())
	})

	t.Run("one of two active admins can be deleted", func(t *testing.T) {
		f := newUserFixture()
		u := newUser(f)
		f.users.LockActiveAdminsFunc = func(context.Context) ([]int64, error) { return []int64{u.ID, 50}, nil }
		_, err := f.svc.Delete(ctx, adminID, "", u.ID, "")
		require.NoError(t, err)
	})

	t.Run("a missing user is ErrUserNotFound", func(t *testing.T) {
		f := newUserFixture()
		_, err := f.svc.Delete(ctx, adminID, "", 404, "")
		assert.ErrorIs(t, err, user.ErrUserNotFound)
	})

	t.Run("an already-deleted user is a quiet success that writes and audits nothing", func(t *testing.T) {
		f := newUserFixture()
		u := newUser(f)
		at := time.Now().Add(-time.Hour)
		u.DeletedAt = &at
		f.users.SoftDeleteFunc = func(context.Context, int64, int64, string) (*user.User, error) {
			t.Error("an already-deleted user was written")
			return nil, nil
		}
		got, err := f.svc.Delete(ctx, adminID, "", u.ID, "again")
		require.NoError(t, err)
		assert.Equal(t, &at, got.DeletedAt)
		assert.Empty(t, f.audit.Entries())
	})
}

func TestUserService_Restore(t *testing.T) {
	ctx := context.Background()

	t.Run("clears the mark and audits who deleted them, when and why; the status stays", func(t *testing.T) {
		f := newUserFixture()
		u, err := f.users.Create(ctx, user.NewUser{Email: "u@example.com", Name: "U", PasswordHash: "keep"})
		require.NoError(t, err)
		u.Status = user.UserStatusInactive
		at, by, reason := time.Now().Add(-time.Hour), int64(7), "left"
		u.DeletedAt, u.DeletedBy, u.DeletedReason = &at, &by, &reason

		got, err := f.svc.Restore(ctx, adminID, "10.0.0.3", u.ID)
		require.NoError(t, err)
		assert.Nil(t, got.DeletedAt)
		assert.Equal(t, user.UserStatusInactive, got.Status)

		entries := f.audit.Entries()
		require.Len(t, entries, 1)
		assert.Equal(t, audit.ActionRestoreUser, entries[0].Action)
		assert.Equal(t, adminID, *entries[0].PerformedBy)
		assert.Equal(t, map[string]any{"deleted_at": &at, "deleted_by": &by, "deleted_reason": &reason}, entries[0].Details)
	})

	t.Run("a user who is not deleted is a quiet success that writes and audits nothing", func(t *testing.T) {
		f := newUserFixture()
		u, err := f.users.Create(ctx, user.NewUser{Email: "u@example.com", Name: "U", PasswordHash: "keep"})
		require.NoError(t, err)
		f.users.RestoreFunc = func(context.Context, int64) (*user.User, error) {
			t.Error("a user who is not deleted was written")
			return nil, nil
		}
		_, err = f.svc.Restore(ctx, adminID, "", u.ID)
		require.NoError(t, err)
		assert.Empty(t, f.audit.Entries())
	})

	t.Run("a missing user is ErrUserNotFound", func(t *testing.T) {
		f := newUserFixture()
		_, err := f.svc.Restore(ctx, adminID, "", 404)
		assert.ErrorIs(t, err, user.ErrUserNotFound)
	})
}

func TestUserService_Create_DeletedEmailPointsToRestore(t *testing.T) {
	f := newUserFixture()
	u, err := f.users.Create(context.Background(), user.NewUser{Email: "pastor@example.com", Name: "Old", PasswordHash: "x"})
	require.NoError(t, err)
	at := time.Now()
	u.DeletedAt = &at

	_, err = f.svc.Create(context.Background(), adminID, "", createReq())
	assert.ErrorIs(t, err, user.ErrEmailBelongsToDeletedUser)
}
