package user

import (
	"time"

	"github.com/ssr0016/pastoral-reports/backend/internal/accesscontrol"
)

// Swagger names are pinned (@name) to the ones these types had in package model,
// so the generated frontend types do not change with the package.

type User struct {
	ID                  int64      `json:"id" db:"id"`
	Email               string     `json:"email" db:"email"`
	Name                string     `json:"name" db:"name"`
	PasswordHash        string     `json:"-" db:"password_hash"`
	RoleID              int64      `json:"role_id" db:"role_id"`
	EmailVerified       bool       `json:"email_verified" db:"email_verified"`
	EmailVerifiedAt     *time.Time `json:"email_verified_at,omitempty" db:"email_verified_at"`
	FailedLoginAttempts int        `json:"-" db:"failed_login_attempts"`
	LockedUntil         *time.Time `json:"locked_until,omitempty" db:"locked_until"`
	CreatedAt           time.Time  `json:"created_at" db:"created_at"`
	UpdatedAt           time.Time  `json:"updated_at" db:"updated_at"`

	// Status is one of UserStatuses; only an active account can sign in (D-A10).
	Status string `json:"status" db:"status"`

	// TokensValidAfter: an access token signed before it is refused. Only
	// GetWithRole loads it (the per-request check); the zero value never refuses.
	TokensValidAfter time.Time `json:"-" db:"tokens_valid_after"`

	// Profile defaults for the report form; nil when the admin has not set them.
	// A report's worker is the user's name, so there is no separate worker.
	AreaAssignment *string `json:"area_assignment,omitempty" db:"area_assignment"`
	ChurchName     *string `json:"church_name,omitempty" db:"church_name"`

	// Role is populated on demand (not always loaded)
	Role *accesscontrol.Role `json:"role,omitempty"`
}

type UserResponse struct {
	ID              int64      `json:"id"`
	Email           string     `json:"email"`
	Name            string     `json:"name"`
	RoleID          int64      `json:"role_id"`
	EmailVerified   bool       `json:"email_verified"`
	EmailVerifiedAt *time.Time `json:"email_verified_at,omitempty"`
	CreatedAt       time.Time  `json:"created_at"`
	Status          string     `json:"status"`
	AreaAssignment  string     `json:"area_assignment,omitempty"`
	ChurchName      string     `json:"church_name,omitempty"`
} // @name github_com_ssr0016_pastoral-reports_backend_internal_model.UserResponse

// NewUser is what the repository needs to insert an account. A zero RoleID
// means the default role; empty profile text is stored as NULL.
type NewUser struct {
	Email, Name, PasswordHash  string
	RoleID                     int64
	AreaAssignment, ChurchName string
}

// UserProfile is the editable part of an account (not the password or role).
type UserProfile struct {
	Name, AreaAssignment, ChurchName string
}

func (u *User) ToResponse() UserResponse {
	return UserResponse{
		ID:              u.ID,
		Email:           u.Email,
		Name:            u.Name,
		RoleID:          u.RoleID,
		EmailVerified:   u.EmailVerified,
		EmailVerifiedAt: u.EmailVerifiedAt,
		CreatedAt:       u.CreatedAt,
		Status:          u.Status,
		AreaAssignment:  deref(u.AreaAssignment),
		ChurchName:      deref(u.ChurchName),
	}
}

// CreateUserRequest is the body of POST /admin/users. A missing role_id means
// the default role. Profile text may be empty.
type CreateUserRequest struct {
	Email    string `json:"email" validate:"required,email"`
	Name     string `json:"name" validate:"required,min=2,max=100"`
	Password string `json:"password" validate:"required,strongpass"`
	// ConfirmPassword must equal Password exactly; it is checked and dropped, never stored.
	ConfirmPassword string `json:"confirm_password" validate:"required,eqfield=Password"`
	RoleID          *int64 `json:"role_id" validate:"omitempty,gt=0"`
	AreaAssignment  string `json:"area_assignment" validate:"max=150"`
	ChurchName      string `json:"church_name" validate:"max=150"`
} // @name github_com_ssr0016_pastoral-reports_backend_internal_model.CreateUserRequest

// UpdateUserRequest is the body of PUT /admin/users/:id. It replaces the name
// and the profile text; the password and role are changed elsewhere.
type UpdateUserRequest struct {
	Name           string `json:"name" validate:"required,min=2,max=100"`
	AreaAssignment string `json:"area_assignment" validate:"max=150"`
	ChurchName     string `json:"church_name" validate:"max=150"`
} // @name github_com_ssr0016_pastoral-reports_backend_internal_model.UpdateUserRequest

// ChangeStatusRequest is the body of PUT /admin/users/:id/status. The list must
// match UserStatuses (a test compares it with chk_users_status).
type ChangeStatusRequest struct {
	Status string `json:"status" validate:"required,oneof=active inactive disciplinary deceased"`
} // @name github_com_ssr0016_pastoral-reports_backend_internal_model.ChangeStatusRequest

// AdminResetPasswordRequest is the body of POST /admin/users/:id/reset-password. There is
// no current password: an admin does not know it. The rules are those of change-password
// (22d): strong enough, at most 72 bytes (checked in the service, in bytes), and the
// confirmation equal to the password.
type AdminResetPasswordRequest struct {
	NewPassword     string `json:"new_password" validate:"required,strongpass"`
	ConfirmPassword string `json:"confirm_password" validate:"required,eqfield=NewPassword"`
} // @name github_com_ssr0016_pastoral-reports_backend_internal_model.AdminResetPasswordRequest

func deref(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}
