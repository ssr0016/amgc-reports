// Package accesscontrol is roles and the permissions granted to them. Role writes
// go through RoleService, which audits them; reads call the stores directly.
// acimpl implements them.
package accesscontrol

import (
	"context"
	"errors"
	"fmt"
)

// ErrProtectedRole is returned (wrapped) when a write targets a built-in role
// (BuiltInRoles). Test for it with errors.Is.
var ErrProtectedRole = errors.New("protected role")

var (
	// ErrRoleNotFound is returned when the role to change does not exist.
	ErrRoleNotFound = errors.New("role not found")
	// ErrRoleNameTaken is returned when another role already has the name.
	ErrRoleNameTaken = errors.New("role name already exists")
	// ErrUnknownPermission is returned when a permission id does not exist.
	ErrUnknownPermission = errors.New("unknown permission")
	// ErrRoleInUse is matched by every *RoleInUseError.
	ErrRoleInUse = errors.New("role in use")
	// ErrInvalidReassignRole is returned when the role to move users to is the
	// role being deleted, admin (D-A22) or does not exist.
	ErrInvalidReassignRole = errors.New("invalid role to reassign users to")
)

// RoleInUseError is returned when a role still held by Users accounts (deleted
// ones included: they keep the foreign key) is deleted.
type RoleInUseError struct{ Users int64 }

func (e *RoleInUseError) Error() string { return fmt.Sprintf("role is assigned to %d users", e.Users) }

// Is makes errors.Is(err, ErrRoleInUse) true.
func (e *RoleInUseError) Is(target error) bool { return target == ErrRoleInUse }

// RoleService is how an admin creates, edits and deletes roles (D-A21). Each
// change and its audit row are written in one transaction.
type RoleService interface {
	Create(ctx context.Context, actorID int64, ip string, req CreateRoleRequest) (*Role, error)
	Update(ctx context.Context, actorID int64, ip string, id int64, req UpdateRoleRequest) (*Role, error)
	// Delete removes a custom role. Its users are first moved to reassignTo; a zero
	// reassignTo refuses a role anyone holds.
	Delete(ctx context.Context, actorID int64, ip string, id, reassignTo int64) error
}

// ErrDefaultRoleMissing is returned (wrapped) when the default role Register
// assigns does not exist. Test for it with errors.Is.
var ErrDefaultRoleMissing = errors.New("default role missing")

// RoleStore defines the interface for role data access.
type RoleStore interface {
	Create(ctx context.Context, req CreateRoleRequest) (*Role, error)
	GetByID(ctx context.Context, id int64) (*Role, error)
	// GetForUpdate is GetByID with the row locked until the transaction in ctx ends;
	// the lock also holds off a user being given the role.
	GetForUpdate(ctx context.Context, id int64) (*Role, error)
	GetByName(ctx context.Context, name string) (*Role, error)
	List(ctx context.Context) ([]Role, error)
	// ListWithPagination fills each role's UserCount.
	ListWithPagination(ctx context.Context, page, limit int) ([]Role, int64, error)
	// Update replaces the name and description; a built-in role is ErrProtectedRole.
	Update(ctx context.Context, id int64, req UpdateRoleRequest) (*Role, error)
	// Delete removes the role; a built-in role is ErrProtectedRole.
	Delete(ctx context.Context, id int64) error
	// CountUsers counts every account holding the role, deleted ones included.
	CountUsers(ctx context.Context, roleID int64) (int64, error)
	GetPermissions(ctx context.Context, roleID int64) ([]Permission, error)
	// ReplacePermissions makes ids the role's whole permission set.
	ReplacePermissions(ctx context.Context, roleID int64, ids []int64) error
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
