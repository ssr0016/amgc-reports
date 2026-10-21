package authimpl

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"golang.org/x/crypto/bcrypt"

	"github.com/ssr0016/pastoral-reports/backend/internal/accesscontrol"
	"github.com/ssr0016/pastoral-reports/backend/internal/audit"
	"github.com/ssr0016/pastoral-reports/backend/internal/auth"
	"github.com/ssr0016/pastoral-reports/backend/internal/auth/token"
	"github.com/ssr0016/pastoral-reports/backend/internal/database"
	"github.com/ssr0016/pastoral-reports/backend/internal/request"
	"github.com/ssr0016/pastoral-reports/backend/internal/user"
)

const (
	maxFailedLoginAttempts = 5
	lockoutDuration        = 15 * time.Minute
)

type service struct {
	userRepo            user.Store
	roleRepo            accesscontrol.RoleStore // for Me; nil in callers that never call it
	verificationService auth.VerificationService
	audit               audit.Store       // for ChangePassword; nil in callers that never call it
	tx                  database.TxRunner // likewise
	log                 *slog.Logger

	// Set by WithTokens; nil in callers that never start or refresh a sign-in.
	tokens   *token.Manager
	refresh  auth.RefreshStore
	tokenSet TokenSettings
}

// NewService returns the auth.Service. issuer, refresh and set enable sign-in
// sessions; a service built with nil ones (most tests) cannot start or refresh one.
func NewService(
	userRepo user.Store,
	roleRepo accesscontrol.RoleStore,
	verificationService auth.VerificationService,
	audit audit.Store,
	tx database.TxRunner,
	log *slog.Logger,
	issuer *token.Manager,
	refresh auth.RefreshStore,
	set TokenSettings,
) auth.Service {
	return newService(userRepo, roleRepo, verificationService, audit, tx, log).withTokens(issuer, refresh, set)
}

// newService is NewService with the concrete type and no sessions, for tests.
func newService(
	userRepo user.Store,
	roleRepo accesscontrol.RoleStore,
	verificationService auth.VerificationService,
	audit audit.Store,
	tx database.TxRunner,
	log *slog.Logger,
) *service {
	return &service{
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
// is auth.ErrUnauthorized, so a session outliving its user stops working.
func (s *service) Me(ctx context.Context, userID int64) (*auth.MeResponse, error) {
	user, err := s.userRepo.GetWithRole(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("get user: %w", err)
	}
	if user == nil || !user.CanSignIn() || accessRevoked(ctx, user) {
		return nil, auth.ErrUnauthorized
	}
	me := &auth.MeResponse{
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

func (s *service) Login(ctx context.Context, email, password string) (*user.User, error) {
	usr, err := s.userRepo.GetByEmail(ctx, email)
	if err != nil {
		return nil, fmt.Errorf("get user: %w", err)
	}
	// A deleted user looks exactly like an unknown email (D-A11): no lockout
	// message, no failed attempt recorded.
	if usr == nil || usr.DeletedAt != nil {
		return nil, auth.ErrInvalidCredentials
	}

	// Check if account is locked
	if usr.LockedUntil != nil && usr.LockedUntil.After(time.Now()) {
		return nil, auth.ErrAccountLocked
	}

	// Verify password
	if err := bcrypt.CompareHashAndPassword([]byte(usr.PasswordHash), []byte(password)); err != nil {
		// Record failed attempt
		if err := s.userRepo.RecordFailedLogin(ctx, usr.ID, maxFailedLoginAttempts, lockoutDuration); err != nil {
			// Still return invalid credentials, but the attempt did not count
			// toward lockout, so make that visible.
			s.log.Warn("failed to record failed login", "user_id", usr.ID, "error", err)
		}
		return nil, auth.ErrInvalidCredentials
	}

	// Only now, with the right password, is the status told. A refused login does
	// not reset the failed-attempt counter and starts no session.
	if !usr.CanSignIn() {
		return nil, statusError(usr.Status)
	}

	// Successful login — reset failed attempts
	if err := s.userRepo.ResetLoginAttempts(ctx, usr.ID); err != nil {
		// Don't fail a login that already succeeded.
		s.log.Warn("failed to reset login attempts", "user_id", usr.ID, "error", err)
	}

	return usr, nil
}

// ChangePassword changes the signed-in user's own password. keepFamily is the
// sign-in they are using (the access token's sid); their other sign-ins end with the change.
//
// The current password is checked against the same failure counter as login, so
// a stolen session cannot use this to guess it. The "same as current" rule
// is applied only after the current password is verified: before that, its
// answer would tell the caller what the current password is.
func (s *service) ChangePassword(ctx context.Context, userID int64, ip, keepFamily, current, next string) error {
	usr, err := s.userRepo.GetWithRole(ctx, userID)
	if err != nil {
		return fmt.Errorf("get user: %w", err)
	}
	if usr == nil || accessRevoked(ctx, usr) {
		return auth.ErrUnauthorized
	}
	if usr.LockedUntil != nil && usr.LockedUntil.After(time.Now()) {
		return auth.ErrAccountLocked
	}

	if err := bcrypt.CompareHashAndPassword([]byte(usr.PasswordHash), []byte(current)); err != nil {
		if err := s.userRepo.RecordFailedLogin(ctx, usr.ID, maxFailedLoginAttempts, lockoutDuration); err != nil {
			s.log.Warn("failed to record failed login", "user_id", usr.ID, "error", err)
		}
		return auth.ErrInvalidCredentials
	}
	if err := s.userRepo.ResetLoginAttempts(ctx, usr.ID); err != nil {
		s.log.Warn("failed to reset login attempts", "user_id", usr.ID, "error", err)
	}

	if next == current {
		return auth.ErrPasswordUnchanged
	}
	if len(next) > auth.MaxPasswordBytes {
		return auth.ErrPasswordTooLong
	}
	hash, err := bcrypt.GenerateFromPassword([]byte(next), bcrypt.DefaultCost)
	if err != nil {
		return fmt.Errorf("hash password: %w", err)
	}

	err = s.tx.InTransaction(ctx, func(ctx context.Context) error {
		ended, err := s.userRepo.UpdatePasswordKeepSession(ctx, usr.ID, string(hash), keepFamily)
		if err != nil {
			return err
		}
		return s.audit.Insert(ctx, user.AuditLog(usr, ip, audit.ActionChangePassword, usr,
			map[string]any{"sessions_ended": ended}))
	})
	if errors.Is(err, database.ErrNotFound) {
		return auth.ErrUnauthorized // the user was deleted meanwhile
	}
	if err != nil {
		return fmt.Errorf("change password: %w", err)
	}
	s.log.Info("password changed", "user_id", usr.ID)
	return nil
}

// accessRevoked reports whether the request's access token was signed before the
// user's tokens_valid_after. A caller with no token in ctx (a test) is not revoked.
func accessRevoked(ctx context.Context, usr *user.User) bool {
	caller, ok := request.UserFrom(ctx)
	return ok && usr.AccessRevoked(caller.IssuedAt)
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
	case user.UserStatusDisciplinary:
		return auth.ErrAccountUnderReview
	case user.UserStatusDeceased:
		return auth.ErrAccountClosed
	default:
		return auth.ErrAccountInactive
	}
}
