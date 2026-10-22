package userimpl

import (
	"context"
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ssr0016/pastoral-reports/backend/internal/accesscontrol"
	"github.com/ssr0016/pastoral-reports/backend/internal/audit"
	"github.com/ssr0016/pastoral-reports/backend/internal/user"
)

func TestUserService_ChangeRole(t *testing.T) {
	ctx := context.Background()
	// roleFixture has the actor (adminID) and a target holding roleID. GetWithRole
	// fills Role with the stub's role-<id> name, except role 1, which is "admin".
	roleFixture := func(roleID int64) (userFixture, *user.User) {
		f := newUserFixture()
		f.svc = NewService(f.users, stubRoleLookup{known: roleSet{1: true, 7: true, 8: true}}, f.audit, fakeTx{})
		u, err := f.users.Create(ctx, user.NewUser{Email: "u@example.com", Name: "U", RoleID: roleID})
		require.NoError(t, err)
		f.users.GetWithRoleFunc = func(_ context.Context, id int64) (*user.User, error) {
			if id == adminID {
				return &user.User{ID: adminID, Name: "Admin", Role: &accesscontrol.Role{Name: "admin"}}, nil
			}
			got, err := f.users.GetByID(ctx, id)
			if got == nil || err != nil {
				return got, err
			}
			c := *got
			name := fmt.Sprintf("role-%d", c.RoleID)
			if c.RoleID == 1 {
				name = "admin"
			}
			c.Role = &accesscontrol.Role{ID: c.RoleID, Name: name}
			return &c, nil
		}
		return f, u
	}

	t.Run("changes the role and writes one audit row with role names", func(t *testing.T) {
		f, u := roleFixture(7)
		got, err := f.svc.ChangeRole(ctx, adminID, "10.0.0.3", u.ID, 8)
		require.NoError(t, err)

		assert.Equal(t, int64(8), got.RoleID)
		entries := f.audit.Entries()
		require.Len(t, entries, 1)
		assert.Equal(t, audit.ActionChangeUserRole, entries[0].Action)
		assert.Equal(t, audit.TargetTypeUser, entries[0].TargetType)
		assert.Equal(t, "u@example.com", *entries[0].TargetName)
		assert.Equal(t, map[string]any{"from": "role-7", "to": "role-8"}, entries[0].Details)
	})

	t.Run("the same role is a quiet success with no audit row", func(t *testing.T) {
		f, u := roleFixture(7)
		f.users.UpdateRoleFunc = func(context.Context, int64, int64) error {
			t.Error("an unchanged role was written")
			return nil
		}
		_, err := f.svc.ChangeRole(ctx, adminID, "", u.ID, 7)
		require.NoError(t, err)
		assert.Empty(t, f.audit.Entries())
	})

	t.Run("your own account is refused", func(t *testing.T) {
		f, _ := roleFixture(7)
		_, err := f.svc.ChangeRole(ctx, adminID, "", adminID, 7)
		require.ErrorIs(t, err, user.ErrCannotChangeOwnRole)
		assert.Empty(t, f.audit.Entries())
	})

	t.Run("the last active admin cannot be moved off admin", func(t *testing.T) {
		f, u := roleFixture(1)
		f.users.LockActiveAdminsFunc = func(context.Context) ([]int64, error) { return []int64{u.ID}, nil }
		_, err := f.svc.ChangeRole(ctx, adminID, "", u.ID, 7)
		require.ErrorIs(t, err, user.ErrLastActiveAdmin)
		assert.Empty(t, f.audit.Entries())
	})

	t.Run("an admin who is not the last one can be moved off admin", func(t *testing.T) {
		f, u := roleFixture(1)
		f.users.LockActiveAdminsFunc = func(context.Context) ([]int64, error) { return []int64{adminID, u.ID}, nil }
		_, err := f.svc.ChangeRole(ctx, adminID, "", u.ID, 7)
		require.NoError(t, err)
		require.Len(t, f.audit.Entries(), 1)
	})

	t.Run("an unknown role is ErrRoleNotFound", func(t *testing.T) {
		f, u := roleFixture(7)
		_, err := f.svc.ChangeRole(ctx, adminID, "", u.ID, 404)
		require.ErrorIs(t, err, user.ErrRoleNotFound)
	})

	t.Run("an unknown user is ErrUserNotFound", func(t *testing.T) {
		f, _ := roleFixture(7)
		_, err := f.svc.ChangeRole(ctx, adminID, "", 12345, 7)
		require.ErrorIs(t, err, user.ErrUserNotFound)
	})
}
