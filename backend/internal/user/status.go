package user

import "time"

// Account statuses (decision D-A10), as stored in users.status (chk_users_status).
const (
	UserStatusActive       = "active"
	UserStatusInactive     = "inactive"
	UserStatusDisciplinary = "disciplinary"
	UserStatusDeceased     = "deceased"
)

// UserStatuses is the source of truth for chk_users_status; a test compares it
// with the CHECK constraint. Adding a status also needs a migration.
var UserStatuses = []string{UserStatusActive, UserStatusInactive, UserStatusDisciplinary, UserStatusDeceased}

// CanSignIn reports whether the account may sign in or keep a session. An empty
// Status is treated as active: the column is NOT NULL, so empty only comes from a
// hand-built value (a test fixture). Any other value is refused, never let through.
func (u *User) CanSignIn() bool {
	return u.Status == "" || u.Status == UserStatusActive
}

// AccessRevoked reports whether an access token signed at iat was invalidated by
// a later password change, reset or status change (D3).
func (u *User) AccessRevoked(iat time.Time) bool {
	return !iat.IsZero() && iat.Before(u.TokensValidAfter.Truncate(time.Millisecond))
}
