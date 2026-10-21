// Package accesscontrol is roles and the permissions granted to them. It has no
// service: the handlers in rest call the stores directly. acimpl implements them.
package accesscontrol

import (
	"context"
	"errors"
)

// ErrProtectedRole is returned (wrapped) when a delete or rename targets the
// default role, which Register depends on. Test for it with errors.Is.
var ErrProtectedRole = errors.New("protected role")

// ErrDefaultRoleMissing is returned (wrapped) when the default role Register
// assigns does not exist. Test for it with errors.Is.
var ErrDefaultRoleMissing = errors.New("default role missing")

// RoleStore defines the interface for role data access.
type RoleStore interface {
	Create(ctx context.Context, req CreateRoleRequest) (*Role, error)
	GetByID(ctx context.Context, id int64) (*Role, error)
	GetByName(ctx context.Context, name string) (*Role, error)
	List(ctx context.Context) ([]Role, error)
	ListWithPagination(ctx context.Context, page, limit int) ([]Role, int64, error)
	Update(ctx context.Context, id int64, req UpdateRoleRequest) (*Role, error)
	Delete(ctx context.Context, id int64) error
	GetPermissions(ctx context.Context, roleID int64) ([]Permission, error)
	AssignPermission(ctx context.Context, roleID, permissionID int64) error
	RevokePermission(ctx context.Context, roleID, permissionID int64) error
}

// PermissionStore defines the interface for permission data access.
type PermissionStore interface {
	Create(ctx context.Context, name, resource, action string) (*Permission, error)
	GetByID(ctx context.Context, id int64) (*Permission, error)
	GetByName(ctx context.Context, name string) (*Permission, error)
	List(ctx context.Context) ([]Permission, error)
	ListWithPagination(ctx context.Context, resource string, page, limit int) ([]Permission, int64, error)
	ListByResource(ctx context.Context, resource string) ([]Permission, error)
	Delete(ctx context.Context, id int64) error
}
