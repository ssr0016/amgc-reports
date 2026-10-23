package acimpl

import (
	"context"
	"errors"
	"fmt"
	"maps"
	"slices"
	"strconv"

	"github.com/ssr0016/pastoral-reports/backend/internal/accesscontrol"
	"github.com/ssr0016/pastoral-reports/backend/internal/audit"
	"github.com/ssr0016/pastoral-reports/backend/internal/auth"
	"github.com/ssr0016/pastoral-reports/backend/internal/database"
	"github.com/ssr0016/pastoral-reports/backend/internal/user"
)

// service writes roles and their audit rows in one transaction (D-A21). Who may
// call it is decided by the route (admin role and roles:write), not here.
type service struct {
	roles accesscontrol.RoleStore
	perms accesscontrol.PermissionStore
	users user.Store
	audit audit.Store
	tx    database.TxRunner
}

// NewService returns the accesscontrol.RoleService over its stores.
func NewService(
	roles accesscontrol.RoleStore,
	perms accesscontrol.PermissionStore,
	users user.Store,
	audit audit.Store,
	tx database.TxRunner,
) accesscontrol.RoleService {
	return &service{roles: roles, perms: perms, users: users, audit: audit, tx: tx}
}

// Create adds a role with its permissions.
func (s *service) Create(ctx context.Context, actorID int64, ip string, req accesscontrol.CreateRoleRequest) (*accesscontrol.Role, error) {
	actor, err := s.actor(ctx, actorID)
	if err != nil {
		return nil, err
	}

	var role *accesscontrol.Role
	err = s.tx.InTransaction(ctx, func(ctx context.Context) error {
		names, err := s.permissionNames(ctx, req.PermissionIDs)
		if err != nil {
			return err
		}
		if role, err = s.roles.Create(ctx, req); err != nil {
			return err
		}
		if err := s.roles.ReplacePermissions(ctx, role.ID, req.PermissionIDs); err != nil {
			return err
		}
		return s.audit.Insert(ctx, roleAuditLog(actor, ip, audit.ActionCreateRole, role,
			map[string]any{"name": role.Name, "permissions": sorted(slices.Collect(maps.Values(names)))}))
	})
	if err != nil {
		return nil, roleError(err, "create role")
	}
	return s.withPermissions(ctx, role)
}

// Update replaces a role's name, description and permission set. A request that
// changes nothing is a quiet success that writes and audits nothing.
func (s *service) Update(ctx context.Context, actorID int64, ip string, id int64, req accesscontrol.UpdateRoleRequest) (*accesscontrol.Role, error) {
	actor, err := s.actor(ctx, actorID)
	if err != nil {
		return nil, err
	}

	var role *accesscontrol.Role
	err = s.tx.InTransaction(ctx, func(ctx context.Context) error {
		var err error
		role, err = s.update(ctx, actor, ip, id, req)
		return err
	})
	if err != nil {
		return nil, roleError(err, "update role")
	}
	return s.withPermissions(ctx, role)
}

// update is the body of Update's transaction; ctx carries it.
func (s *service) update(ctx context.Context, actor *user.User, ip string, id int64, req accesscontrol.UpdateRoleRequest) (*accesscontrol.Role, error) {
	current, err := s.roles.GetForUpdate(ctx, id)
	if err != nil {
		return nil, err
	}
	if current == nil {
		return nil, accesscontrol.ErrRoleNotFound
	}
	names, err := s.permissionNames(ctx, req.PermissionIDs)
	if err != nil {
		return nil, err
	}
	held, err := s.roles.GetPermissions(ctx, id)
	if err != nil {
		return nil, err
	}
	added, removed := diffPermissions(held, names)
	renamed := current.Name != req.Name
	if !renamed && current.Description == req.Description && len(added) == 0 && len(removed) == 0 {
		return current, nil
	}
	if accesscontrol.IsBuiltIn(current.Name) {
		return nil, fmt.Errorf("update role: %w: %s", accesscontrol.ErrProtectedRole, current.Name)
	}

	// The row is locked above, so Update cannot find it gone.
	role, err := s.roles.Update(ctx, id, req)
	if err != nil {
		return nil, err
	}
	if len(added) > 0 || len(removed) > 0 {
		if err := s.roles.ReplacePermissions(ctx, id, req.PermissionIDs); err != nil {
			return nil, err
		}
	}
	details := map[string]any{"added": sorted(added), "removed": sorted(removed)}
	if renamed {
		details["from"], details["to"] = current.Name, role.Name
	}
	return role, s.audit.Insert(ctx, roleAuditLog(actor, ip, audit.ActionUpdateRole, role, details))
}

// diffPermissions names the permissions in want (id to name) that held lacks, and
// those held has that want lacks.
func diffPermissions(held []accesscontrol.Permission, want map[int64]string) (added, removed []string) {
	for _, p := range held {
		if _, keep := want[p.ID]; !keep {
			removed = append(removed, p.Name)
		}
	}
	for id, name := range want {
		if !slices.ContainsFunc(held, func(p accesscontrol.Permission) bool { return p.ID == id }) {
			added = append(added, name)
		}
	}
	return added, removed
}

// Delete removes a role, first moving any account holding it to reassignTo. The
// role's row is locked first, which also holds off a user being given the role
// until the delete commits; the target is locked so it cannot vanish meanwhile.
func (s *service) Delete(ctx context.Context, actorID int64, ip string, id, reassignTo int64) error {
	actor, err := s.actor(ctx, actorID)
	if err != nil {
		return err
	}

	err = s.tx.InTransaction(ctx, func(ctx context.Context) error {
		role, err := s.roles.GetForUpdate(ctx, id)
		if err != nil {
			return err
		}
		if role == nil {
			return accesscontrol.ErrRoleNotFound
		}
		if accesscontrol.IsBuiltIn(role.Name) {
			return fmt.Errorf("delete role: %w: %s", accesscontrol.ErrProtectedRole, role.Name)
		}
		n, err := s.roles.CountUsers(ctx, id)
		if err != nil {
			return err
		}
		details := map[string]any{"name": role.Name}
		if n > 0 {
			if reassignTo == 0 {
				return &accesscontrol.RoleInUseError{Users: n}
			}
			to, err := s.reassign(ctx, actor, ip, role, reassignTo)
			if err != nil {
				return err
			}
			details["reassigned_to"] = to
		}
		if err := s.roles.Delete(ctx, id); err != nil {
			return err
		}
		return s.audit.Insert(ctx, roleAuditLog(actor, ip, audit.ActionDeleteRole, role, details))
	})
	if err != nil {
		return roleError(err, "delete role")
	}
	return nil
}

// reassign moves every holder of from to the role toID, with a CHANGE_USER_ROLE
// row each, and returns that role's name.
func (s *service) reassign(ctx context.Context, actor *user.User, ip string, from *accesscontrol.Role, toID int64) (string, error) {
	if toID == from.ID {
		return "", accesscontrol.ErrInvalidReassignRole
	}
	to, err := s.roles.GetForUpdate(ctx, toID)
	if err != nil {
		return "", err
	}
	// admin is refused so one request can never promote a whole role (D-A22).
	if to == nil || to.Name == "admin" {
		return "", accesscontrol.ErrInvalidReassignRole
	}
	moved, err := s.users.ReassignRole(ctx, from.ID, to.ID)
	if err != nil {
		return "", err
	}
	for i := range moved {
		if err := s.audit.Insert(ctx, user.AuditLog(actor, ip, audit.ActionChangeUserRole, &moved[i],
			map[string]any{"from": from.Name, "to": to.Name})); err != nil {
			return "", err
		}
	}
	return to.Name, nil
}

// permissionNames maps each id to its permission's name; an id that does not
// exist is ErrUnknownPermission.
func (s *service) permissionNames(ctx context.Context, ids []int64) (map[int64]string, error) {
	all, err := s.perms.List(ctx)
	if err != nil {
		return nil, fmt.Errorf("list permissions: %w", err)
	}
	names := make(map[int64]string, len(ids))
	for _, id := range ids {
		i := slices.IndexFunc(all, func(p accesscontrol.Permission) bool { return p.ID == id })
		if i < 0 {
			return nil, fmt.Errorf("%w: %d", accesscontrol.ErrUnknownPermission, id)
		}
		names[id] = all[i].Name
	}
	return names, nil
}

func (s *service) withPermissions(ctx context.Context, role *accesscontrol.Role) (*accesscontrol.Role, error) {
	perms, err := s.roles.GetPermissions(ctx, role.ID)
	if err != nil {
		return nil, fmt.Errorf("load permissions: %w", err)
	}
	role.Permissions = perms
	if role.UserCount, err = s.roles.CountUsers(ctx, role.ID); err != nil {
		return nil, err
	}
	return role, nil
}

// actor is the admin making the change, for the audit row: the caller
// middleware.LoadCaller put in ctx, or else loaded. A session whose user is gone
// is unauthorized.
func (s *service) actor(ctx context.Context, id int64) (*user.User, error) {
	if c, ok := user.CallerFrom(ctx, id); ok {
		return c.User, nil
	}
	u, err := s.users.GetWithRole(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("get actor: %w", err)
	}
	if u == nil {
		return nil, auth.ErrUnauthorized
	}
	return u, nil
}

// roleError keeps the service's own errors and maps the store's: a duplicate name
// and a permission deleted mid-write. Anything else is wrapped with op.
func roleError(err error, op string) error {
	switch {
	case errors.Is(err, database.ErrUniqueViolation):
		return accesscontrol.ErrRoleNameTaken
	case errors.Is(err, database.ErrForeignKeyViolation) && op == "delete role":
		// The count above holds under the row lock; this is a safety net.
		return &accesscontrol.RoleInUseError{}
	case errors.Is(err, database.ErrForeignKeyViolation):
		return accesscontrol.ErrUnknownPermission
	case errors.Is(err, database.ErrNotFound):
		return accesscontrol.ErrRoleNotFound
	case errors.Is(err, accesscontrol.ErrRoleNotFound), errors.Is(err, accesscontrol.ErrProtectedRole),
		errors.Is(err, accesscontrol.ErrRoleInUse), errors.Is(err, accesscontrol.ErrUnknownPermission),
		errors.Is(err, accesscontrol.ErrInvalidReassignRole),
		errors.Is(err, auth.ErrUnauthorized):
		return err
	}
	return fmt.Errorf("%s: %w", op, err)
}

// roleAuditLog is the audit row for a change to role made by actor. Details hold
// names, never the description (free text).
func roleAuditLog(actor *user.User, ip, action string, role *accesscontrol.Role, details map[string]any) audit.Log {
	e := user.AuditLog(actor, ip, action, &user.User{}, details)
	id := strconv.FormatInt(role.ID, 10)
	e.TargetType, e.TargetID, e.TargetName = audit.TargetTypeRole, &id, &role.Name
	return e
}

// sorted returns a sorted copy that is never nil, so the audit row stores [] not null.
func sorted(s []string) []string {
	out := append([]string{}, s...)
	slices.Sort(out)
	return out
}
