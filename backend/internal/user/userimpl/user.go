package userimpl

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"strings"

	"golang.org/x/crypto/bcrypt"

	"github.com/ssr0016/pastoral-reports/backend/internal/accesscontrol"
	"github.com/ssr0016/pastoral-reports/backend/internal/audit"
	"github.com/ssr0016/pastoral-reports/backend/internal/auth"
	"github.com/ssr0016/pastoral-reports/backend/internal/database"
	"github.com/ssr0016/pastoral-reports/backend/internal/user"
)

// service is how an admin creates and edits accounts. Each change and its
// audit row are written in one transaction. Who may call it is decided by the
// route (RequirePermission), not here.
type service struct {
	users user.Store
	roles accesscontrol.RoleStore
	audit audit.Store
	tx    database.TxRunner
}

// NewService returns the user.Service over its stores.
func NewService(
	users user.Store,
	roles accesscontrol.RoleStore,
	audit audit.Store,
	tx database.TxRunner,
) user.Service {
	return &service{users: users, roles: roles, audit: audit, tx: tx}
}

// Create adds an account. A nil RoleID means the default role.
func (s *service) Create(ctx context.Context, actorID int64, ip string, req user.CreateUserRequest) (*user.User, error) {
	actor, err := s.actor(ctx, actorID)
	if err != nil {
		return nil, err
	}

	var roleID int64
	details := map[string]any{}
	if req.RoleID != nil {
		role, err := s.roles.GetByID(ctx, *req.RoleID)
		if err != nil {
			return nil, fmt.Errorf("get role: %w", err)
		}
		if role == nil {
			return nil, user.ErrRoleNotFound
		}
		roleID = role.ID
		details["role_id"] = role.ID
	}

	existing, err := s.users.GetByEmail(ctx, req.Email)
	if err != nil {
		return nil, fmt.Errorf("check email: %w", err)
	}
	if existing != nil {
		if existing.DeletedAt != nil {
			return nil, user.ErrEmailBelongsToDeletedUser
		}
		return nil, user.ErrEmailTaken
	}

	hash, err := bcrypt.GenerateFromPassword([]byte(req.Password), bcrypt.DefaultCost)
	if err != nil {
		return nil, fmt.Errorf("hash password: %w", err)
	}

	var usr *user.User
	err = s.tx.InTransaction(ctx, func(ctx context.Context) error {
		var err error
		usr, err = s.users.Create(ctx, user.NewUser{
			Email: req.Email, Name: req.Name, PasswordHash: string(hash), RoleID: roleID,
			AreaAssignment: req.AreaAssignment, ChurchName: req.ChurchName,
		})
		if err != nil {
			return err
		}
		return s.audit.Insert(ctx, user.AuditLog(actor, ip, audit.ActionCreateUser, usr, details))
	})
	switch {
	case errors.Is(err, database.ErrUniqueViolation):
		// A concurrent create passed the check above; the unique index decides.
		return nil, user.ErrEmailTaken
	case errors.Is(err, database.ErrForeignKeyViolation):
		// The role was deleted between the check and the insert.
		return nil, user.ErrRoleNotFound
	case err != nil:
		return nil, fmt.Errorf("create user: %w", err)
	}
	return usr, nil
}

// Update replaces a user's name and profile text.
func (s *service) Update(ctx context.Context, actorID int64, ip string, id int64, req user.UpdateUserRequest) (*user.User, error) {
	actor, err := s.actor(ctx, actorID)
	if err != nil {
		return nil, err
	}

	var usr *user.User
	err = s.tx.InTransaction(ctx, func(ctx context.Context) error {
		var err error
		usr, err = s.users.UpdateProfile(ctx, id, user.UserProfile(req))
		if err != nil {
			return err
		}
		return s.audit.Insert(ctx, user.AuditLog(actor, ip, audit.ActionUpdateUser, usr, map[string]any{}))
	})
	switch {
	case errors.Is(err, database.ErrNotFound):
		return nil, user.ErrUserNotFound
	case err != nil:
		return nil, fmt.Errorf("update user: %w", err)
	}
	return usr, nil
}

// ChangeStatus sets a user's status (D-A10). The change, the end of the user's
// sessions (for any status but active) and the audit row are one transaction. An
// admin cannot change their own status, and the only active admin cannot be moved
// away from active: the active admins' rows are locked in id order first, so two
// admins changing each other's status at once cannot both pass that check. The
// same status as now is a quiet success that writes and audits nothing.
func (s *service) ChangeStatus(ctx context.Context, actorID int64, ip string, id int64, status string) (*user.User, error) {
	if !slices.Contains(user.UserStatuses, status) {
		return nil, user.ErrInvalidUserStatus
	}
	actor, err := s.actor(ctx, actorID)
	if err != nil {
		return nil, err
	}
	if actorID == id {
		return nil, user.ErrCannotChangeOwnStatus
	}

	var usr *user.User
	err = s.tx.InTransaction(ctx, func(ctx context.Context) error {
		var err error
		usr, err = s.setStatus(ctx, actor, ip, id, status)
		return err
	})
	switch {
	case errors.Is(err, user.ErrLastActiveAdmin):
		return nil, user.ErrLastActiveAdmin
	case errors.Is(err, database.ErrNotFound):
		return nil, user.ErrUserNotFound
	case err != nil:
		return nil, fmt.Errorf("change user status: %w", err)
	}
	return usr, nil
}

// setStatus is the body of ChangeStatus's transaction; ctx carries it.
func (s *service) setStatus(ctx context.Context, actor *user.User, ip string, id int64, status string) (*user.User, error) {
	admins, err := s.users.LockActiveAdmins(ctx)
	if err != nil {
		return nil, err
	}
	// Only the audit's "from" and the unchanged shortcut depend on this read; the lock
	// above guards the admins.
	current, err := s.users.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}
	if current == nil {
		return nil, database.ErrNotFound
	}
	if current.Status == status {
		return current, nil
	}
	if status != user.UserStatusActive && len(admins) == 1 && admins[0] == id {
		return nil, user.ErrLastActiveAdmin
	}

	from := current.Status
	usr, err := s.users.UpdateStatus(ctx, id, status)
	if err != nil {
		return nil, err
	}
	var revoked int64
	if status != user.UserStatusActive {
		if revoked, err = s.users.RevokeSessions(ctx, id); err != nil {
			return nil, err
		}
	}
	err = s.audit.Insert(ctx, user.AuditLog(actor, ip, audit.ActionChangeUserStatus, usr,
		map[string]any{"from": from, "to": status, "sessions_revoked": revoked}))
	if err != nil {
		return nil, err
	}
	return usr, nil
}

// ResetPassword sets a new password for another user (D-A12). The password change, the
// end of every session of that user, the lockout clear and the audit row are one
// transaction. The admin never resets their own account, whatever the body says. The
// 72-byte limit is checked first because bcrypt would otherwise cut the password there.
// Any status and any role may be reset; the status is not changed. The audit details hold
// only how many sessions ended, never the password or its hash.
func (s *service) ResetPassword(ctx context.Context, actorID int64, ip string, id int64, newPassword string) error {
	actor, err := s.actor(ctx, actorID)
	if err != nil {
		return err
	}
	if actorID == id {
		return user.ErrCannotResetOwnPassword
	}
	if len(newPassword) > auth.MaxPasswordBytes {
		return auth.ErrPasswordTooLong
	}
	target, err := s.users.GetByID(ctx, id)
	if err != nil {
		return fmt.Errorf("get user: %w", err)
	}
	if target == nil {
		return user.ErrUserNotFound
	}
	hash, err := bcrypt.GenerateFromPassword([]byte(newPassword), bcrypt.DefaultCost)
	if err != nil {
		return fmt.Errorf("hash password: %w", err)
	}

	err = s.tx.InTransaction(ctx, func(ctx context.Context) error {
		// An empty keep-token: an admin reset has no session to keep, so all of them end.
		ended, err := s.users.UpdatePasswordKeepSession(ctx, id, string(hash), "")
		if err != nil {
			return err
		}
		if err := s.users.ClearLockout(ctx, id); err != nil {
			return err
		}
		return s.audit.Insert(ctx, user.AuditLog(actor, ip, audit.ActionAdminResetPassword, target,
			map[string]any{"sessions_ended": ended}))
	})
	switch {
	case errors.Is(err, database.ErrNotFound):
		return user.ErrUserNotFound
	case err != nil:
		return fmt.Errorf("reset password: %w", err)
	}
	return nil
}

// Delete soft-deletes a user (D-A11): the mark, the end of the user's sessions and the
// audit row are one transaction. An admin cannot delete their own account, and the only
// active admin cannot be deleted: the active admins' rows are locked in id order first,
// as in ChangeStatus. The status is not changed. An already-deleted user is a quiet
// success that writes and audits nothing.
func (s *service) Delete(ctx context.Context, actorID int64, ip string, id int64, reason string) (*user.User, error) {
	actor, err := s.actor(ctx, actorID)
	if err != nil {
		return nil, err
	}
	if actorID == id {
		return nil, user.ErrCannotDeleteSelf
	}
	reason = strings.TrimSpace(reason)

	var usr *user.User
	err = s.tx.InTransaction(ctx, func(ctx context.Context) error {
		var err error
		usr, err = s.softDelete(ctx, actor, ip, id, reason)
		return err
	})
	switch {
	case errors.Is(err, user.ErrLastActiveAdmin):
		return nil, user.ErrLastActiveAdmin
	case errors.Is(err, database.ErrNotFound):
		return nil, user.ErrUserNotFound
	case err != nil:
		return nil, fmt.Errorf("delete user: %w", err)
	}
	return usr, nil
}

// softDelete is the body of Delete's transaction; ctx carries it.
func (s *service) softDelete(ctx context.Context, actor *user.User, ip string, id int64, reason string) (*user.User, error) {
	admins, err := s.users.LockActiveAdmins(ctx)
	if err != nil {
		return nil, err
	}
	current, err := s.users.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}
	if current == nil {
		return nil, database.ErrNotFound
	}
	if current.DeletedAt != nil {
		return current, nil
	}
	if len(admins) == 1 && admins[0] == id {
		return nil, user.ErrLastActiveAdmin
	}

	usr, err := s.users.SoftDelete(ctx, id, actor.ID, reason)
	if err != nil {
		return nil, err
	}
	revoked, err := s.users.RevokeSessions(ctx, id)
	if err != nil {
		return nil, err
	}
	var auditReason any // JSON null when no reason was given
	if reason != "" {
		auditReason = reason
	}
	err = s.audit.Insert(ctx, user.AuditLog(actor, ip, audit.ActionDeleteUser, usr,
		map[string]any{"reason": auditReason, "sessions_revoked": revoked}))
	if err != nil {
		return nil, err
	}
	return usr, nil
}

// Restore clears a user's deleted mark (D-A11). The change and the audit row, which
// keeps who deleted them, when and why, are one transaction; the restorer is the audit
// row's performer. The status is not changed. A user who is not deleted is a quiet
// success that writes and audits nothing.
func (s *service) Restore(ctx context.Context, actorID int64, ip string, id int64) (*user.User, error) {
	actor, err := s.actor(ctx, actorID)
	if err != nil {
		return nil, err
	}

	var usr *user.User
	err = s.tx.InTransaction(ctx, func(ctx context.Context) error {
		current, err := s.users.GetByID(ctx, id)
		if err != nil {
			return err
		}
		if current == nil {
			return database.ErrNotFound
		}
		if current.DeletedAt == nil {
			usr = current
			return nil
		}
		// Copied before the store call: the mock store returns the same row it changes.
		details := map[string]any{
			"deleted_at": current.DeletedAt, "deleted_by": current.DeletedBy, "deleted_reason": current.DeletedReason,
		}
		if usr, err = s.users.Restore(ctx, id); err != nil {
			return err
		}
		return s.audit.Insert(ctx, user.AuditLog(actor, ip, audit.ActionRestoreUser, usr, details))
	})
	switch {
	case errors.Is(err, database.ErrNotFound):
		return nil, user.ErrUserNotFound
	case err != nil:
		return nil, fmt.Errorf("restore user: %w", err)
	}
	return usr, nil
}

// actor loads the admin making the change, for the audit row. A session whose
// user is gone is unauthorized.
func (s *service) actor(ctx context.Context, id int64) (*user.User, error) {
	u, err := s.users.GetWithRole(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("get actor: %w", err)
	}
	if u == nil {
		return nil, auth.ErrUnauthorized
	}
	return u, nil
}
