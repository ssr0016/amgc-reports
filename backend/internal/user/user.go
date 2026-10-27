// Package user is the accounts admins manage: profile, role and status.
// Callers depend on the interfaces here; userimpl implements them and rest
// serves them.
package user

import (
	"context"
	"errors"
	"time"

	"github.com/ssr0016/pastoral-reports/backend/pkg/pagination"
)

var (
	// ErrEmailTaken is returned when another account already has the email.
	ErrEmailTaken = errors.New("email already taken")
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
	// ErrInvalidUserStatus: the status is not one of UserStatuses.
	ErrInvalidUserStatus = errors.New("invalid user status")
	// ErrCannotChangeOwnRole: an admin never changes their own role (D-A21).
	ErrCannotChangeOwnRole = errors.New("cannot change your own role")
	// ErrCannotDeleteSelf: an admin never deletes their own account (D-A11).
	ErrCannotDeleteSelf = errors.New("cannot delete your own account")
	// ErrEmailBelongsToDeletedUser: the email is reserved by a deleted account, which
	// can be restored instead (D-A11).
	ErrEmailBelongsToDeletedUser = errors.New("email belongs to a deleted user")
)

// Service is how an admin creates and edits accounts. Each change and its
// audit row are written in one transaction.
type Service interface {
	Create(ctx context.Context, actorID int64, ip string, req CreateUserRequest) (*User, error)
	Update(ctx context.Context, actorID int64, ip string, id int64, req UpdateUserRequest) (*User, error)
	ChangeStatus(ctx context.Context, actorID int64, ip string, id int64, status string) (*User, error)
	ResetPassword(ctx context.Context, actorID int64, ip string, id int64, newPassword string) error
	Delete(ctx context.Context, actorID int64, ip string, id int64, reason string) (*User, error)
	Restore(ctx context.Context, actorID int64, ip string, id int64) (*User, error)
	ChangeRole(ctx context.Context, actorID int64, ip string, id, roleID int64) (*User, error)
}

// Store defines the interface for user data access.
type Store interface {
	GetByID(ctx context.Context, id int64) (*User, error)
	GetByEmail(ctx context.Context, email string) (*User, error)
	CreateWithPassword(ctx context.Context, email, name, hash string) (*User, error)
	Create(ctx context.Context, n NewUser) (*User, error)
	UpdateProfile(ctx context.Context, id int64, p UserProfile) (*User, error)
	// UpdateAvatar commits the new URL and returns the previous URL for safe cleanup.
	// A missing user is ErrNotFound.
	UpdateAvatar(ctx context.Context, id int64, url string) (*User, string, error)
	// List returns one page of users newest first, with the total that match; an empty
	// status means every status and a zero roleID every role. Deleted users are left
	// out unless status is "deleted", which returns only them.
	List(ctx context.Context, emailFilter, status string, roleID int64, pg pagination.Params) ([]User, int64, error)
	MarkEmailVerified(ctx context.Context, userID int64) error
	UpdatePassword(ctx context.Context, userID int64, hash string) error
	UpdatePasswordKeepSession(ctx context.Context, userID int64, hash, keepFamily string) (int64, error)
	RecordFailedLogin(ctx context.Context, userID int64, maxAttempts int, lockDuration time.Duration) error
	ResetLoginAttempts(ctx context.Context, userID int64) error
	// ClearLockout zeroes the failed-login counter and the lock on q, so an admin reset can
	// do it in the same transaction as the password change. A missing user is ErrNotFound.
	ClearLockout(ctx context.Context, userID int64) error
	UpdateRole(ctx context.Context, userID, roleID int64) error
	// ReassignRole moves every account holding from, deleted ones included, to to
	// and returns them.
	ReassignRole(ctx context.Context, from, to int64) ([]User, error)
	GetWithRole(ctx context.Context, id int64) (*User, error)
	// GetCaller is GetWithRole plus the role's permission names, in one query.
	// A missing user is nil, nil.
	GetCaller(ctx context.Context, id int64) (*Caller, error)
	UpdateStatus(ctx context.Context, id int64, status string) (*User, error)
	RevokeSessions(ctx context.Context, userID int64) (int64, error)
	LockActiveAdmins(ctx context.Context) ([]int64, error)
	LockUser(ctx context.Context, userID int64) error
	// SoftDelete marks the user deleted by deletedBy (an empty reason is stored as NULL);
	// Restore clears that mark. A missing user is ErrNotFound.
	SoftDelete(ctx context.Context, id, deletedBy int64, reason string) (*User, error)
	Restore(ctx context.Context, id int64) (*User, error)
}
