package service

import (
	"context"
	"errors"
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ssr0016/pastoral-reports/backend/internal/model"
	"github.com/ssr0016/pastoral-reports/backend/internal/repository"
)

func TestUserService_ChangeStatus(t *testing.T) {
	ctx := context.Background()
	newUser := func(f userFixture) *model.User {
		u, err := f.users.Create(ctx, nil, model.NewUser{Email: "u@example.com", Name: "U", PasswordHash: "keep"})
		require.NoError(t, err)
		return u
	}
	// admins sets who the active-admin lock reports.
	admins := func(f userFixture, ids ...int64) {
		f.users.LockActiveAdminsFunc = func(context.Context, repository.Querier) ([]int64, error) { return ids, nil }
	}

	t.Run("moves a user away from active: updates, ends sessions, one audit row", func(t *testing.T) {
		f := newUserFixture()
		u := newUser(f)
		f.users.RevokeSessionsFunc = func(context.Context, repository.Querier, int64) (int64, error) { return 3, nil }

		got, err := f.svc.ChangeStatus(ctx, adminID, "10.0.0.2", u.ID, model.UserStatusInactive)
		require.NoError(t, err)

		assert.Equal(t, model.UserStatusInactive, got.Status)
		entries := f.audit.Entries()
		require.Len(t, entries, 1)
		assert.Equal(t, model.AuditChangeUserStatus, entries[0].Action)
		assert.Equal(t, model.AuditTargetTypeUser, entries[0].TargetType)
		assert.Equal(t, fmt.Sprint(u.ID), *entries[0].TargetID)
		assert.Equal(t, "u@example.com", *entries[0].TargetName)
		assert.Equal(t, adminID, *entries[0].PerformedBy)
		assert.Equal(t, map[string]any{"from": "active", "to": "inactive", "sessions_revoked": int64(3)}, entries[0].Details)
	})

	t.Run("moving back to active does not end sessions", func(t *testing.T) {
		f := newUserFixture()
		u := newUser(f)
		u.Status = model.UserStatusDeceased
		f.users.RevokeSessionsFunc = func(context.Context, repository.Querier, int64) (int64, error) {
			t.Error("sessions were revoked on a move to active")
			return 0, nil
		}

		got, err := f.svc.ChangeStatus(ctx, adminID, "", u.ID, model.UserStatusActive)
		require.NoError(t, err)

		assert.Equal(t, model.UserStatusActive, got.Status)
		require.Len(t, f.audit.Entries(), 1)
	})

	t.Run("the same status is a quiet success: nothing written, no audit row", func(t *testing.T) {
		f := newUserFixture()
		u := newUser(f)
		f.users.UpdateStatusFunc = func(context.Context, repository.Querier, int64, string) (*model.User, error) {
			t.Error("an unchanged status was written")
			return nil, nil
		}

		got, err := f.svc.ChangeStatus(ctx, adminID, "", u.ID, model.UserStatusActive)
		require.NoError(t, err)

		assert.Equal(t, model.UserStatusActive, got.Status)
		assert.Empty(t, f.audit.Entries())
	})

	t.Run("your own account is refused, even to the status it already has", func(t *testing.T) {
		f := newUserFixture()
		for _, status := range []string{model.UserStatusInactive, model.UserStatusActive} {
			_, err := f.svc.ChangeStatus(ctx, adminID, "", adminID, status)
			assert.ErrorIs(t, err, ErrCannotChangeOwnStatus, status)
		}
		assert.Empty(t, f.audit.Entries())
	})

	t.Run("the only active admin cannot be moved away from active", func(t *testing.T) {
		f := newUserFixture()
		u := newUser(f)
		admins(f, u.ID)

		_, err := f.svc.ChangeStatus(ctx, adminID, "", u.ID, model.UserStatusDisciplinary)

		assert.ErrorIs(t, err, ErrLastActiveAdmin)
		assert.Empty(t, f.audit.Entries())
		assert.Equal(t, model.UserStatusActive, u.Status, "the refused change was written")
	})

	t.Run("an admin who is not the last one can be moved", func(t *testing.T) {
		f := newUserFixture()
		u := newUser(f)
		admins(f, u.ID, 500)

		_, err := f.svc.ChangeStatus(ctx, adminID, "", u.ID, model.UserStatusInactive)

		require.NoError(t, err)
	})

	t.Run("a status that is not one of the four is refused", func(t *testing.T) {
		f := newUserFixture()
		u := newUser(f)
		_, err := f.svc.ChangeStatus(ctx, adminID, "", u.ID, "retired")
		assert.ErrorIs(t, err, ErrInvalidUserStatus)
		assert.Empty(t, f.audit.Entries())
	})

	t.Run("unknown user writes nothing", func(t *testing.T) {
		f := newUserFixture()
		got, err := f.svc.ChangeStatus(ctx, adminID, "", 404, model.UserStatusInactive)
		assert.Nil(t, got)
		assert.ErrorIs(t, err, ErrUserNotFound)
		assert.Empty(t, f.audit.Entries())
	})

	t.Run("audit failure fails the change", func(t *testing.T) {
		f := newUserFixture()
		u := newUser(f)
		f.audit.InsertFunc = func(context.Context, repository.Querier, model.AuditLog) error { return errors.New("audit down") }

		_, err := f.svc.ChangeStatus(ctx, adminID, "", u.ID, model.UserStatusInactive)

		assert.ErrorContains(t, err, "audit down")
	})

	t.Run("the actor no longer exists", func(t *testing.T) {
		f := newUserFixture()
		u := newUser(f)
		_, err := f.svc.ChangeStatus(ctx, 12345, "", u.ID, model.UserStatusInactive)
		assert.ErrorIs(t, err, ErrUnauthorized)
	})
}
