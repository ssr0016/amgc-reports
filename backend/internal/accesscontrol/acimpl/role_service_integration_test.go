//go:build integration

package acimpl

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ssr0016/pastoral-reports/backend/internal/accesscontrol"
	"github.com/ssr0016/pastoral-reports/backend/internal/audit"
	"github.com/ssr0016/pastoral-reports/backend/internal/audit/auditimpl"
	"github.com/ssr0016/pastoral-reports/backend/internal/database"
	"github.com/ssr0016/pastoral-reports/backend/internal/testutil"
	"github.com/ssr0016/pastoral-reports/backend/internal/user/userimpl"
)

// failingAudit refuses every row, so the write it is called after must roll back.
type failingAudit struct{}

func (failingAudit) Insert(context.Context, audit.Log) error { return errors.New("audit down") }

func TestRoleService_Postgres(t *testing.T) {
	tdb := testutil.SetupPostgres(t)
	tdb.MigrateWithGoose(t) // the real migrations: audit_logs and its CHECK come from them
	db := &database.DB{Pool: tdb.Pool, Builder: database.NewStatementBuilder()}
	roles := NewRoleStore(db)
	perms := NewPermissionStore(db)
	users := userimpl.NewStore(db)
	ctx := context.Background()

	read, err := perms.Create(ctx, "reports:read_any", "reports", "read_any")
	require.NoError(t, err)
	approve, err := perms.Create(ctx, "reports:approve", "reports", "approve")
	require.NoError(t, err)
	actor, err := users.CreateWithPassword(ctx, "actor@example.com", "Actor", "hash")
	require.NoError(t, err)
	svc := NewService(roles, perms, users, auditimpl.NewStore(db), db)

	auditCount := func(action string) int {
		var n int
		require.NoError(t, tdb.Pool.QueryRow(ctx, `SELECT COUNT(*) FROM audit_logs WHERE action = $1`, action).Scan(&n))
		return n
	}

	t.Run("create, update and delete, one audit row each", func(t *testing.T) {
		role, err := svc.Create(ctx, actor.ID, "", accesscontrol.CreateRoleRequest{Name: "Reviewer", PermissionIDs: []int64{read.ID}})
		require.NoError(t, err)
		require.Len(t, role.Permissions, 1)

		got, err := svc.Update(ctx, actor.ID, "", role.ID, accesscontrol.UpdateRoleRequest{
			Name: "Senior Reviewer", PermissionIDs: []int64{approve.ID},
		})
		require.NoError(t, err)
		require.Equal(t, "Senior Reviewer", got.Name)
		require.Len(t, got.Permissions, 1)
		assert.Equal(t, "reports:approve", got.Permissions[0].Name)

		_, err = svc.Update(ctx, actor.ID, "", role.ID, accesscontrol.UpdateRoleRequest{
			Name: "Senior Reviewer", PermissionIDs: []int64{approve.ID},
		})
		require.NoError(t, err)

		require.NoError(t, svc.Delete(ctx, actor.ID, "", role.ID, 0))
		assert.Equal(t, 1, auditCount(audit.ActionCreateRole))
		assert.Equal(t, 1, auditCount(audit.ActionUpdateRole), "the no-op update writes no row")
		assert.Equal(t, 1, auditCount(audit.ActionDeleteRole))
	})

	t.Run("an audit failure rolls the role change back", func(t *testing.T) {
		failing := NewService(roles, perms, users, failingAudit{}, db)
		_, err := failing.Create(ctx, actor.ID, "", accesscontrol.CreateRoleRequest{Name: "Ghost", PermissionIDs: []int64{read.ID}})
		require.Error(t, err)
		gone, err := roles.GetByName(ctx, "Ghost")
		require.NoError(t, err)
		assert.Nil(t, gone)

		kept, err := svc.Create(ctx, actor.ID, "", accesscontrol.CreateRoleRequest{Name: "Kept", PermissionIDs: []int64{read.ID}})
		require.NoError(t, err)
		_, err = failing.Update(ctx, actor.ID, "", kept.ID, accesscontrol.UpdateRoleRequest{Name: "Renamed", PermissionIDs: []int64{}})
		require.Error(t, err)
		after, err := roles.GetByID(ctx, kept.ID)
		require.NoError(t, err)
		assert.Equal(t, "Kept", after.Name)
		held, err := roles.GetPermissions(ctx, kept.ID)
		require.NoError(t, err)
		assert.Len(t, held, 1, "the permission change rolls back too")
	})

	t.Run("a role held only by a deleted account cannot be deleted, and the list counts it", func(t *testing.T) {
		role, err := svc.Create(ctx, actor.ID, "", accesscontrol.CreateRoleRequest{Name: "Held"})
		require.NoError(t, err)
		u, err := users.CreateWithPassword(ctx, "held@example.com", "Held", "hash")
		require.NoError(t, err)
		require.NoError(t, users.UpdateRole(ctx, u.ID, role.ID))
		_, err = users.SoftDelete(ctx, u.ID, actor.ID, "")
		require.NoError(t, err)

		var inUse *accesscontrol.RoleInUseError
		require.ErrorAs(t, svc.Delete(ctx, actor.ID, "", role.ID, 0), &inUse)
		assert.Equal(t, int64(1), inUse.Users)

		list, _, err := roles.ListWithPagination(ctx, 1, 100)
		require.NoError(t, err)
		for _, r := range list {
			if r.ID == role.ID {
				assert.Equal(t, int64(1), r.UserCount)
			}
		}
	})

	t.Run("reassign_to moves every holder, deleted ones included, then deletes", func(t *testing.T) {
		from, err := svc.Create(ctx, actor.ID, "", accesscontrol.CreateRoleRequest{Name: "Leaving"})
		require.NoError(t, err)
		to, err := svc.Create(ctx, actor.ID, "", accesscontrol.CreateRoleRequest{Name: "Staying"})
		require.NoError(t, err)
		for _, email := range []string{"move1@example.com", "move2@example.com"} {
			u, err := users.CreateWithPassword(ctx, email, "Move", "hash")
			require.NoError(t, err)
			require.NoError(t, users.UpdateRole(ctx, u.ID, from.ID))
			if email == "move2@example.com" {
				_, err = users.SoftDelete(ctx, u.ID, actor.ID, "")
				require.NoError(t, err)
			}
		}

		require.NoError(t, svc.Delete(ctx, actor.ID, "", from.ID, to.ID))

		gone, err := roles.GetByID(ctx, from.ID)
		require.NoError(t, err)
		assert.Nil(t, gone)
		n, err := roles.CountUsers(ctx, to.ID)
		require.NoError(t, err)
		assert.Equal(t, int64(2), n)
	})

	t.Run("the store refuses to change any built-in role", func(t *testing.T) {
		for _, name := range accesscontrol.BuiltInRoles {
			role, err := roles.GetByName(ctx, name)
			require.NoError(t, err)
			if role == nil {
				role, err = roles.Create(ctx, accesscontrol.CreateRoleRequest{Name: name})
				require.NoError(t, err)
			}
			_, err = roles.Update(ctx, role.ID, accesscontrol.UpdateRoleRequest{Name: name + "-x"})
			require.ErrorIs(t, err, accesscontrol.ErrProtectedRole, name)
			require.ErrorIs(t, roles.Delete(ctx, role.ID), accesscontrol.ErrProtectedRole, name)
		}
	})
}
