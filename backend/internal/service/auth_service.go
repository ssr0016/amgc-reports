package service

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"golang.org/x/crypto/bcrypt"

	"github.com/ssr0016/pastoral-reports/backend/internal/model"
	"github.com/ssr0016/pastoral-reports/backend/internal/repository"
)

var (
	ErrInvalidCredentials = errors.New("invalid email or password")
	ErrEmailTaken         = errors.New("email already taken")
	ErrUnauthorized       = errors.New("unauthorized")
	ErrAccountLocked      = errors.New("account is locked, try again later")
	// The account is not active (D-A10). Only returned after the password was right,
	// so they cannot be used to find out which emails have an account.
	ErrAccountInactive    = errors.New("account is inactive")
	ErrAccountUnderReview = errors.New("account is under review")
	ErrAccountClosed      = errors.New("account is closed")
	// ErrPasswordUnchanged: the new password equals the current one.
	ErrPasswordUnchanged = errors.New("new password must differ from the current one")
	// ErrPasswordTooLong: bcrypt reads 72 bytes and no more, so a longer password
	// would be silently cut. It counts bytes, not characters.
	ErrPasswordTooLong = errors.New("password must be at most 72 bytes")
)

const (
	maxFailedLoginAttempts = 5
	lockoutDuration        = 15 * time.Minute
	maxPasswordBytes       = 72
)

type AuthService struct {
	userRepo            repository.UserRepository
	roleRepo            repository.RoleRepository // for Me; nil in callers that never call it
	verificationService *VerificationService
	audit               repository.AuditRepository // for ChangePassword; nil in callers that never call it
	tx                  repository.TxRunner        // likewise
	log                 *slog.Logger
}

func NewAuthService(
	userRepo repository.UserRepository,
	roleRepo repository.RoleRepository,
	verificationService *VerificationService,
	audit repository.AuditRepository,
	tx repository.TxRunner,
	log *slog.Logger,
) *AuthService {
	return &AuthService{
		userRepo:            userRepo,
		roleRepo:            roleRepo,
		verificationService: verificationService,
		audit:               audit,
		tx:                  tx,
		log:                 log,
	}
}

// Me returns the user's name, email, role and permissions, read live from the
// database (a role change shows on the next call). A user that no longer exists
// is ErrUnauthorized, so a session outliving its user stops working.
func (s *AuthService) Me(ctx context.Context, userID int64) (*model.MeResponse, error) {
	user, err := s.userRepo.GetWithRole(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("get user: %w", err)
	}
	if user == nil || !user.CanSignIn() {
		return nil, ErrUnauthorized
	}
	me := &model.MeResponse{
		UserID: user.ID, Name: user.Name, Email: user.Email, Permissions: []string{},
		AreaAssignment: deref(user.AreaAssignment), ChurchName: deref(user.ChurchName),
	}
	if user.Role != nil {
		me.Role = user.Role.Name
	}
	if user.RoleID != 0 {
		perms, err := s.roleRepo.GetPermissions(ctx, user.RoleID)
		if err != nil {
			return nil, fmt.Errorf("get permissions: %w", err)
		}
		for _, p := range perms {
			me.Permissions = append(me.Permissions, p.Name)
		}
	}
	return me, nil
}

func (s *AuthService) Login(ctx context.Context, email, password string) (*model.User, error) {
	user, err := s.userRepo.GetByEmail(ctx, email)
	if err != nil {
		return nil, fmt.Errorf("get user: %w", err)
	}
	if user == nil {
		return nil, ErrInvalidCredentials
	}

	// Check if account is locked
	if user.LockedUntil != nil && user.LockedUntil.After(time.Now()) {
		return nil, ErrAccountLocked
	}

	// Verify password
	if err := bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(password)); err != nil {
		// Record failed attempt
		if err := s.userRepo.RecordFailedLogin(ctx, user.ID, maxFailedLoginAttempts, lockoutDuration); err != nil {
			// Still return invalid credentials, but the attempt did not count
			// toward lockout, so make that visible.
			s.log.Warn("failed to record failed login", "user_id", user.ID, "error", err)
		}
		return nil, ErrInvalidCredentials
	}

	// Only now, with the right password, is the status told. A refused login does
	// not reset the failed-attempt counter and starts no session.
	if !user.CanSignIn() {
		return nil, statusError(user.Status)
	}

	// Successful login — reset failed attempts
	if err := s.userRepo.ResetLoginAttempts(ctx, user.ID); err != nil {
		// Don't fail a login that already succeeded.
		s.log.Warn("failed to reset login attempts", "user_id", user.ID, "error", err)
	}

	return user, nil
}

// ChangePassword changes the signed-in user's own password. keepToken is the
// session they are using; their other sessions end with the change.
//
// The current password is checked against the same failure counter as login, so
// a stolen session cannot use this to guess it. The "same as current" rule
// is applied only after the current password is verified: before that, its
// answer would tell the caller what the current password is.
func (s *AuthService) ChangePassword(ctx context.Context, userID int64, ip, keepToken, current, next string) error {
	user, err := s.userRepo.GetWithRole(ctx, userID)
	if err != nil {
		return fmt.Errorf("get user: %w", err)
	}
	if user == nil {
		return ErrUnauthorized
	}
	if user.LockedUntil != nil && user.LockedUntil.After(time.Now()) {
		return ErrAccountLocked
	}

	if err := bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(current)); err != nil {
		if err := s.userRepo.RecordFailedLogin(ctx, user.ID, maxFailedLoginAttempts, lockoutDuration); err != nil {
			s.log.Warn("failed to record failed login", "user_id", user.ID, "error", err)
		}
		return ErrInvalidCredentials
	}
	if err := s.userRepo.ResetLoginAttempts(ctx, user.ID); err != nil {
		s.log.Warn("failed to reset login attempts", "user_id", user.ID, "error", err)
	}

	if next == current {
		return ErrPasswordUnchanged
	}
	if len(next) > maxPasswordBytes {
		return ErrPasswordTooLong
	}
	hash, err := bcrypt.GenerateFromPassword([]byte(next), bcrypt.DefaultCost)
	if err != nil {
		return fmt.Errorf("hash password: %w", err)
	}

	err = s.tx.WithTx(ctx, func(q repository.Querier) error {
		ended, err := s.userRepo.UpdatePasswordKeepSession(ctx, q, user.ID, string(hash), keepToken)
		if err != nil {
			return err
		}
		return s.audit.Insert(ctx, q, userAudit(user, ip, model.AuditChangePassword, user,
			map[string]any{"sessions_ended": ended}))
	})
	if errors.Is(err, repository.ErrNotFound) {
		return ErrUnauthorized // the user was deleted meanwhile
	}
	if err != nil {
		return fmt.Errorf("change password: %w", err)
	}
	s.log.Info("password changed", "user_id", user.ID)
	return nil
}

func deref(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}

// statusError is the login refusal for a non-active status. A status this code
// does not know is treated as inactive: it is refused, never let through.
func statusError(status string) error {
	switch status {
	case model.UserStatusDisciplinary:
		return ErrAccountUnderReview
	case model.UserStatusDeceased:
		return ErrAccountClosed
	default:
		return ErrAccountInactive
	}
}
