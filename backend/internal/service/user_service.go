package service

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"strconv"

	"golang.org/x/crypto/bcrypt"

	"github.com/ssr0016/pastoral-reports/backend/internal/model"
	"github.com/ssr0016/pastoral-reports/backend/internal/repository"
)

var (
	// ErrRoleNotFound is returned when the requested role does not exist.
	ErrRoleNotFound = errors.New("role not found")
	// ErrUserNotFound is returned when the user to update does not exist.
	ErrUserNotFound = errors.New("user not found")
	// ErrCannotChangeOwnStatus: an admin never changes their own status (D-A10).
	ErrCannotChangeOwnStatus = errors.New("cannot change your own status")
	// ErrCannotResetOwnPassword: an admin changes their own password on the Change
	// password page, which asks for the current one (D-A12).
	ErrCannotResetOwnPassword = errors.New("cannot reset your own password")
	// ErrLastActiveAdmin: the only active admin cannot be moved away from active.
	ErrLastActiveAdmin = errors.New("at least one active admin is required")
	// ErrInvalidUserStatus: the status is not one of model.UserStatuses.
	ErrInvalidUserStatus = errors.New("invalid user status")
)

// UserService is how an admin creates and edits accounts. Each change and its
// audit row are written in one transaction. Who may call it is decided by the
// route (RequirePermission), not here.
type UserService struct {
	users repository.UserRepository
	roles repository.RoleRepository
	audit repository.AuditRepository
	tx    repository.TxRunner
}

func NewUserService(
	users repository.UserRepository,
	roles repository.RoleRepository,
	audit repository.AuditRepository,
	tx repository.TxRunner,
) *UserService {
	return &UserService{users: users, roles: roles, audit: audit, tx: tx}
}

// Create adds an account. A nil RoleID means the default role.
func (s *UserService) Create(ctx context.Context, actorID int64, ip string, req model.CreateUserRequest) (*model.User, error) {
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
			return nil, ErrRoleNotFound
		}
		roleID = role.ID
		details["role_id"] = role.ID
	}

	existing, err := s.users.GetByEmail(ctx, req.Email)
	if err != nil {
		return nil, fmt.Errorf("check email: %w", err)
	}
	if existing != nil {
		return nil, ErrEmailTaken
	}

	hash, err := bcrypt.GenerateFromPassword([]byte(req.Password), bcrypt.DefaultCost)
	if err != nil {
		return nil, fmt.Errorf("hash password: %w", err)
	}

	var user *model.User
	err = s.tx.InTransaction(ctx, func(ctx context.Context) error {
		var err error
		user, err = s.users.Create(ctx, model.NewUser{
			Email: req.Email, Name: req.Name, PasswordHash: string(hash), RoleID: roleID,
			AreaAssignment: req.AreaAssignment, ChurchName: req.ChurchName,
		})
		if err != nil {
			return err
		}
		return s.audit.Insert(ctx, userAudit(actor, ip, model.AuditCreateUser, user, details))
	})
	switch {
	case errors.Is(err, repository.ErrUniqueViolation):
		// A concurrent create passed the check above; the unique index decides.
		return nil, ErrEmailTaken
	case errors.Is(err, repository.ErrForeignKeyViolation):
		// The role was deleted between the check and the insert.
		return nil, ErrRoleNotFound
	case err != nil:
		return nil, fmt.Errorf("create user: %w", err)
	}
	return user, nil
}

// Update replaces a user's name and profile text.
func (s *UserService) Update(ctx context.Context, actorID int64, ip string, id int64, req model.UpdateUserRequest) (*model.User, error) {
	actor, err := s.actor(ctx, actorID)
	if err != nil {
		return nil, err
	}

	var user *model.User
	err = s.tx.InTransaction(ctx, func(ctx context.Context) error {
		var err error
		user, err = s.users.UpdateProfile(ctx, id, model.UserProfile(req))
		if err != nil {
			return err
		}
		return s.audit.Insert(ctx, userAudit(actor, ip, model.AuditUpdateUser, user, map[string]any{}))
	})
	switch {
	case errors.Is(err, repository.ErrNotFound):
		return nil, ErrUserNotFound
	case err != nil:
		return nil, fmt.Errorf("update user: %w", err)
	}
	return user, nil
}

// ChangeStatus sets a user's status (D-A10). The change, the end of the user's
// sessions (for any status but active) and the audit row are one transaction. An
// admin cannot change their own status, and the only active admin cannot be moved
// away from active: the active admins' rows are locked in id order first, so two
// admins changing each other's status at once cannot both pass that check. The
// same status as now is a quiet success that writes and audits nothing.
func (s *UserService) ChangeStatus(ctx context.Context, actorID int64, ip string, id int64, status string) (*model.User, error) {
	if !slices.Contains(model.UserStatuses, status) {
		return nil, ErrInvalidUserStatus
	}
	actor, err := s.actor(ctx, actorID)
	if err != nil {
		return nil, err
	}
	if actorID == id {
		return nil, ErrCannotChangeOwnStatus
	}

	var user *model.User
	err = s.tx.InTransaction(ctx, func(ctx context.Context) error {
		var err error
		user, err = s.setStatus(ctx, actor, ip, id, status)
		return err
	})
	switch {
	case errors.Is(err, ErrLastActiveAdmin):
		return nil, ErrLastActiveAdmin
	case errors.Is(err, repository.ErrNotFound):
		return nil, ErrUserNotFound
	case err != nil:
		return nil, fmt.Errorf("change user status: %w", err)
	}
	return user, nil
}

// setStatus is the body of ChangeStatus's transaction; ctx carries it.
func (s *UserService) setStatus(ctx context.Context, actor *model.User, ip string, id int64, status string) (*model.User, error) {
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
		return nil, repository.ErrNotFound
	}
	if current.Status == status {
		return current, nil
	}
	if status != model.UserStatusActive && len(admins) == 1 && admins[0] == id {
		return nil, ErrLastActiveAdmin
	}

	from := current.Status
	user, err := s.users.UpdateStatus(ctx, id, status)
	if err != nil {
		return nil, err
	}
	var revoked int64
	if status != model.UserStatusActive {
		if revoked, err = s.users.RevokeSessions(ctx, id); err != nil {
			return nil, err
		}
	}
	err = s.audit.Insert(ctx, userAudit(actor, ip, model.AuditChangeUserStatus, user,
		map[string]any{"from": from, "to": status, "sessions_revoked": revoked}))
	if err != nil {
		return nil, err
	}
	return user, nil
}

// ResetPassword sets a new password for another user (D-A12). The password change, the
// end of every session of that user, the lockout clear and the audit row are one
// transaction. The admin never resets their own account, whatever the body says. The
// 72-byte limit is checked first because bcrypt would otherwise cut the password there.
// Any status and any role may be reset; the status is not changed. The audit details hold
// only how many sessions ended, never the password or its hash.
func (s *UserService) ResetPassword(ctx context.Context, actorID int64, ip string, id int64, newPassword string) error {
	actor, err := s.actor(ctx, actorID)
	if err != nil {
		return err
	}
	if actorID == id {
		return ErrCannotResetOwnPassword
	}
	if len(newPassword) > maxPasswordBytes {
		return ErrPasswordTooLong
	}
	target, err := s.users.GetByID(ctx, id)
	if err != nil {
		return fmt.Errorf("get user: %w", err)
	}
	if target == nil {
		return ErrUserNotFound
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
		return s.audit.Insert(ctx, userAudit(actor, ip, model.AuditAdminResetPassword, target,
			map[string]any{"sessions_ended": ended}))
	})
	switch {
	case errors.Is(err, repository.ErrNotFound):
		return ErrUserNotFound
	case err != nil:
		return fmt.Errorf("reset password: %w", err)
	}
	return nil
}

// actor loads the admin making the change, for the audit row. A session whose
// user is gone is unauthorized.
func (s *UserService) actor(ctx context.Context, id int64) (*model.User, error) {
	u, err := s.users.GetWithRole(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("get actor: %w", err)
	}
	if u == nil {
		return nil, ErrUnauthorized
	}
	return u, nil
}

// userAudit never carries the password or its hash: the target is named by its
// email, and details hold only non-secret facts such as the role.
func userAudit(actor *model.User, ip, action string, target *model.User, details map[string]any) model.AuditLog {
	id := strconv.FormatInt(target.ID, 10)
	e := model.AuditLog{
		PerformedBy: &actor.ID, PerformedByName: &actor.Name,
		Action: action, TargetType: model.AuditTargetTypeUser, TargetID: &id, TargetName: &target.Email,
		Details: details,
	}
	if actor.Role != nil {
		e.PerformedByRole = &actor.Role.Name
	}
	if ip != "" {
		e.IPAddress = &ip
	}
	return e
}
