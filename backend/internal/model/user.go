package model

import "time"

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

	// Role is populated on demand (not always loaded)
	Role *Role `json:"role,omitempty"`
}

type UserResponse struct {
	ID              int64      `json:"id"`
	Email           string     `json:"email"`
	Name            string     `json:"name"`
	RoleID          int64      `json:"role_id"`
	EmailVerified   bool       `json:"email_verified"`
	EmailVerifiedAt *time.Time `json:"email_verified_at,omitempty"`
	CreatedAt       time.Time  `json:"created_at"`
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
	}
}

type RegisterRequest struct {
	Email    string `json:"email" validate:"required,email"`
	Name     string `json:"name" validate:"required,min=2,max=100"`
	Password string `json:"password" validate:"required,strongpass"`
}

type LoginRequest struct {
	Email    string `json:"email" validate:"required,email"`
	Password string `json:"password" validate:"required"`
}
