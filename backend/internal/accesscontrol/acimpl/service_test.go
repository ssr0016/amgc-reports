package acimpl

import (
	"context"
	"errors"
	"slices"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ssr0016/pastoral-reports/backend/internal/accesscontrol"
	"github.com/ssr0016/pastoral-reports/backend/internal/audit"
	"github.com/ssr0016/pastoral-reports/backend/internal/audit/auditimpl"
	"github.com/ssr0016/pastoral-reports/backend/internal/database"
	"github.com/ssr0016/pastoral-reports/backend/internal/user"
	"github.com/ssr0016/pastoral-reports/backend/internal/user/userimpl"
)

const adminID int64 = 1

// fakeTx runs fn directly. The in-memory stores have no transaction to roll back.
type fakeTx struct{}

func (fakeTx) InTransaction(ctx context.Context, fn func(context.Context) error) error {
	return fn(ctx)
}

// memRoles is an in-memory RoleStore holding only what the service touches.
type memRoles struct {
	accesscontrol.RoleStore
	roles  map[int64]*accesscontrol.Role
	grants map[int64][]int64
	users  map[int64]int64 // role id -> accounts holding it
	nextID int64
	writes int
}

func (m *memRoles) GetForUpdate(_ context.Context, id int64) (*accesscontrol.Role, error) {
	if r, ok := m.roles[id]; ok {
		c := *r
		return &c, nil
	}
	return nil, nil
}

func (m *memRoles) Create(_ context.Context, req accesscontrol.CreateRoleRequest) (*accesscontrol.Role, error) {
	for _, r := range m.roles {
		if r.Name == req.Name {
			return nil, database.ErrUniqueViolation
		}
	}
	m.nextID++
	m.writes++
	r := &accesscontrol.Role{ID: m.nextID, Name: req.Name, Description: req.Description}
	m.roles[r.ID] = r
	c := *r
	return &c, nil
}

func (m *memRoles) Update(_ context.Context, id int64, req accesscontrol.UpdateRoleRequest) (*accesscontrol.Role, error) {
	m.writes++
	r := m.roles[id]
	r.Name, r.Description = req.Name, req.Description
	c := *r
	return &c, nil
}

func (m *memRoles) Delete(_ context.Context, id int64) error {
	m.writes++
	delete(m.roles, id)
	return nil
}

func (m *memRoles) CountUsers(_ context.Context, id int64) (int64, error) { return m.users[id], nil }

func (m *memRoles) GetPermissions(_ context.Context, id int64) ([]accesscontrol.Permission, error) {
	var out []accesscontrol.Permission
	for _, pid := range m.grants[id] {
		out = append(out, permByID(pid))
	}
	return out, nil
}

func (m *memRoles) ReplacePermissions(_ context.Context, id int64, ids []int64) error {
	m.writes++
	m.grants[id] = slices.Clone(ids)
	return nil
}

var allPerms = []accesscontrol.Permission{
	{ID: 11, Name: "reports:read_any"}, {ID: 12, Name: "reports:approve"}, {ID: 13, Name: "users:read"},
}

func permByID(id int64) accesscontrol.Permission {
	return allPerms[slices.IndexFunc(allPerms, func(p accesscontrol.Permission) bool { return p.ID == id })]
}

type memPerms struct{ accesscontrol.PermissionStore }

func (memPerms) List(context.Context) ([]accesscontrol.Permission, error) { return allPerms, nil }

type roleFixture struct {
	roles *memRoles
	audit *auditimpl.MockStore
	svc   accesscontrol.RoleService
}

// newRoleFixture has the built-in admin role (1) and a custom "Reviewer" (5)
// holding reports:read_any.
func newRoleFixture() roleFixture {
	roles := &memRoles{
		roles: map[int64]*accesscontrol.Role{
			1: {ID: 1, Name: "admin"},
			5: {ID: 5, Name: "Reviewer", Description: "Reads reports"},
		},
		grants: map[int64][]int64{5: {11}},
		users:  map[int64]int64{},
		nextID: 5,
	}
	users := userimpl.NewMockStore()
	users.GetWithRoleFunc = func(_ context.Context, id int64) (*user.User, error) {
		if id == adminID {
			return &user.User{ID: adminID, Name: "Admin", Role: &accesscontrol.Role{Name: "admin"}}, nil
		}
		return nil, nil
	}
	f := roleFixture{roles: roles, audit: auditimpl.NewMockStore()}
	f.svc = NewService(roles, memPerms{}, users, f.audit, fakeTx{})
	return f
}

func TestRoleService_Create(t *testing.T) {
	ctx := context.Background()

	t.Run("creates the role with its permissions and one audit row", func(t *testing.T) {
		f := newRoleFixture()
		got, err := f.svc.Create(ctx, adminID, "10.0.0.1", accesscontrol.CreateRoleRequest{
			Name: "CGP Coordinator", Description: "free text", PermissionIDs: []int64{12, 11},
		})
		require.NoError(t, err)

		assert.Equal(t, "CGP Coordinator", got.Name)
		assert.ElementsMatch(t, []int64{12, 11}, f.roles.grants[got.ID])
		entries := f.audit.Entries()
		require.Len(t, entries, 1)
		e := entries[0]
		assert.Equal(t, audit.ActionCreateRole, e.Action)
		assert.Equal(t, audit.TargetTypeRole, e.TargetType)
		assert.Equal(t, "CGP Coordinator", *e.TargetName)
		assert.Equal(t, "Admin", *e.PerformedByName)
		assert.Equal(t, "10.0.0.1", *e.IPAddress)
		assert.Equal(t, map[string]any{
			"name": "CGP Coordinator", "permissions": []string{"reports:approve", "reports:read_any"},
		}, e.Details)
	})

	t.Run("an unknown permission id is refused and nothing is written", func(t *testing.T) {
		f := newRoleFixture()
		_, err := f.svc.Create(ctx, adminID, "", accesscontrol.CreateRoleRequest{Name: "X1", PermissionIDs: []int64{99}})
		require.ErrorIs(t, err, accesscontrol.ErrUnknownPermission)
		assert.Zero(t, f.roles.writes)
		assert.Empty(t, f.audit.Entries())
	})

	t.Run("a taken name is ErrRoleNameTaken", func(t *testing.T) {
		f := newRoleFixture()
		_, err := f.svc.Create(ctx, adminID, "", accesscontrol.CreateRoleRequest{Name: "Reviewer"})
		require.ErrorIs(t, err, accesscontrol.ErrRoleNameTaken)
	})

	t.Run("an audit failure fails the create", func(t *testing.T) {
		f := newRoleFixture()
		f.audit.InsertFunc = func(context.Context, audit.Log) error { return errors.New("audit down") }
		_, err := f.svc.Create(ctx, adminID, "", accesscontrol.CreateRoleRequest{Name: "X1"})
		require.Error(t, err)
	})
}

func TestRoleService_Update(t *testing.T) {
	ctx := context.Background()

	t.Run("rename only: from and to, no permission change", func(t *testing.T) {
		f := newRoleFixture()
		got, err := f.svc.Update(ctx, adminID, "", 5, accesscontrol.UpdateRoleRequest{
			Name: "Senior Reviewer", Description: "Reads reports", PermissionIDs: []int64{11},
		})
		require.NoError(t, err)

		assert.Equal(t, "Senior Reviewer", got.Name)
		entries := f.audit.Entries()
		require.Len(t, entries, 1)
		assert.Equal(t, audit.ActionUpdateRole, entries[0].Action)
		assert.Equal(t, map[string]any{
			"from": "Reviewer", "to": "Senior Reviewer", "added": []string{}, "removed": []string{},
		}, entries[0].Details)
	})

	t.Run("permissions added and removed, by name, with no from/to", func(t *testing.T) {
		f := newRoleFixture()
		_, err := f.svc.Update(ctx, adminID, "", 5, accesscontrol.UpdateRoleRequest{
			Name: "Reviewer", Description: "Reads reports", PermissionIDs: []int64{12, 13},
		})
		require.NoError(t, err)

		assert.ElementsMatch(t, []int64{12, 13}, f.roles.grants[5])
		entries := f.audit.Entries()
		require.Len(t, entries, 1)
		assert.Equal(t, map[string]any{
			"added": []string{"reports:approve", "users:read"}, "removed": []string{"reports:read_any"},
		}, entries[0].Details)
	})

	t.Run("no change writes nothing and no audit row", func(t *testing.T) {
		f := newRoleFixture()
		got, err := f.svc.Update(ctx, adminID, "", 5, accesscontrol.UpdateRoleRequest{
			Name: "Reviewer", Description: "Reads reports", PermissionIDs: []int64{11},
		})
		require.NoError(t, err)
		assert.Equal(t, "Reviewer", got.Name)
		assert.Zero(t, f.roles.writes)
		assert.Empty(t, f.audit.Entries())
	})

	t.Run("a built-in role cannot be renamed or have its permissions changed", func(t *testing.T) {
		for name, req := range map[string]accesscontrol.UpdateRoleRequest{
			"rename":      {Name: "boss", PermissionIDs: []int64{}},
			"permissions": {Name: "admin", PermissionIDs: []int64{11}},
			"description": {Name: "admin", Description: "x", PermissionIDs: []int64{}},
		} {
			t.Run(name, func(t *testing.T) {
				f := newRoleFixture()
				_, err := f.svc.Update(ctx, adminID, "", 1, req)
				require.ErrorIs(t, err, accesscontrol.ErrProtectedRole)
				assert.Zero(t, f.roles.writes)
				assert.Empty(t, f.audit.Entries())
			})
		}
	})

	t.Run("a missing role is ErrRoleNotFound", func(t *testing.T) {
		f := newRoleFixture()
		_, err := f.svc.Update(ctx, adminID, "", 404, accesscontrol.UpdateRoleRequest{Name: "X1", PermissionIDs: []int64{}})
		require.ErrorIs(t, err, accesscontrol.ErrRoleNotFound)
	})

	t.Run("an unknown permission id is refused", func(t *testing.T) {
		f := newRoleFixture()
		_, err := f.svc.Update(ctx, adminID, "", 5, accesscontrol.UpdateRoleRequest{Name: "Reviewer", PermissionIDs: []int64{99}})
		require.ErrorIs(t, err, accesscontrol.ErrUnknownPermission)
		assert.Zero(t, f.roles.writes)
	})
}

func TestRoleService_Delete(t *testing.T) {
	ctx := context.Background()

	t.Run("deletes an unused custom role with one audit row", func(t *testing.T) {
		f := newRoleFixture()
		require.NoError(t, f.svc.Delete(ctx, adminID, "", 5))

		assert.NotContains(t, f.roles.roles, int64(5))
		entries := f.audit.Entries()
		require.Len(t, entries, 1)
		assert.Equal(t, audit.ActionDeleteRole, entries[0].Action)
		assert.Equal(t, "Reviewer", *entries[0].TargetName)
		assert.Equal(t, map[string]any{"name": "Reviewer"}, entries[0].Details)
	})

	t.Run("a role with users is refused with the count", func(t *testing.T) {
		f := newRoleFixture()
		f.roles.users[5] = 2
		err := f.svc.Delete(ctx, adminID, "", 5)

		var inUse *accesscontrol.RoleInUseError
		require.ErrorAs(t, err, &inUse)
		assert.Equal(t, int64(2), inUse.Users)
		assert.ErrorIs(t, err, accesscontrol.ErrRoleInUse)
		assert.Zero(t, f.roles.writes)
		assert.Empty(t, f.audit.Entries())
	})

	t.Run("a built-in role is refused", func(t *testing.T) {
		f := newRoleFixture()
		require.ErrorIs(t, f.svc.Delete(ctx, adminID, "", 1), accesscontrol.ErrProtectedRole)
		assert.Zero(t, f.roles.writes)
	})

	t.Run("a missing role is ErrRoleNotFound", func(t *testing.T) {
		f := newRoleFixture()
		require.ErrorIs(t, f.svc.Delete(ctx, adminID, "", 404), accesscontrol.ErrRoleNotFound)
	})
}
