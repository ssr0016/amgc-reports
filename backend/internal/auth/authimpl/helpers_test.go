package authimpl

import (
	"context"

	"github.com/ssr0016/pastoral-reports/backend/internal/accesscontrol"
)

// fakeTx runs fn directly. The in-memory mocks have no transaction to roll back.
type fakeTx struct{}

func (fakeTx) InTransaction(ctx context.Context, fn func(context.Context) error) error {
	return fn(ctx)
}

// stubRoles serves permissions by role ID; other methods panic if called.
type stubRoles struct {
	accesscontrol.RoleStore
	perms map[int64][]string
}

func (s stubRoles) GetPermissions(_ context.Context, roleID int64) ([]accesscontrol.Permission, error) {
	var out []accesscontrol.Permission
	for _, n := range s.perms[roleID] {
		out = append(out, accesscontrol.Permission{Name: n})
	}
	return out, nil
}
