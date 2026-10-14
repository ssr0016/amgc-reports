package service

import (
	"context"
	"errors"
	"fmt"
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
	err = s.tx.WithTx(ctx, func(q repository.Querier) error {
		var err error
		user, err = s.users.Create(ctx, q, model.NewUser{
			Email: req.Email, Name: req.Name, PasswordHash: string(hash), RoleID: roleID,
			Worker: req.Worker, AreaAssignment: req.AreaAssignment, ChurchName: req.ChurchName,
		})
		if err != nil {
			return err
		}
		return s.audit.Insert(ctx, q, userAudit(actor, ip, model.AuditCreateUser, user, details))
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
	err = s.tx.WithTx(ctx, func(q repository.Querier) error {
		var err error
		user, err = s.users.UpdateProfile(ctx, q, id, model.UserProfile(req))
		if err != nil {
			return err
		}
		return s.audit.Insert(ctx, q, userAudit(actor, ip, model.AuditUpdateUser, user, map[string]any{}))
	})
	switch {
	case errors.Is(err, repository.ErrNotFound):
		return nil, ErrUserNotFound
	case err != nil:
		return nil, fmt.Errorf("update user: %w", err)
	}
	return user, nil
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
