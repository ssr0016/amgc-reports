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

	// Profile defaults for the report form; nil when the admin has not set them.
	Worker         *string `json:"worker,omitempty" db:"worker"`
	AreaAssignment *string `json:"area_assignment,omitempty" db:"area_assignment"`
	ChurchName     *string `json:"church_name,omitempty" db:"church_name"`

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
	Worker          string     `json:"worker,omitempty"`
	AreaAssignment  string     `json:"area_assignment,omitempty"`
	ChurchName      string     `json:"church_name,omitempty"`
}

// NewUser is what the repository needs to insert an account. A zero RoleID
// means the default role; empty profile text is stored as NULL.
type NewUser struct {
	Email, Name, PasswordHash          string
	RoleID                             int64
	Worker, AreaAssignment, ChurchName string
}

// UserProfile is the editable part of an account (not the password or role).
type UserProfile struct {
	Name, Worker, AreaAssignment, ChurchName string
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
		Worker:          deref(u.Worker),
		AreaAssignment:  deref(u.AreaAssignment),
		ChurchName:      deref(u.ChurchName),
	}
}

// MeResponse is the signed-in user's identity, role and permissions, so a client
// can show or hide actions. The server still enforces every permission.
type MeResponse struct {
	UserID      int64    `json:"user_id"`
	Name        string   `json:"name"`
	Email       string   `json:"email"`
	Role        string   `json:"role"`
	Permissions []string `json:"permissions"`
	// Report form defaults; empty when the admin has not set them.
	Worker         string `json:"worker"`
	AreaAssignment string `json:"area_assignment"`
	ChurchName     string `json:"church_name"`
}

// CreateUserRequest is the body of POST /admin/users. A missing role_id means
// the default role. Profile text may be empty.
type CreateUserRequest struct {
	Email          string `json:"email" validate:"required,email"`
	Name           string `json:"name" validate:"required,min=2,max=100"`
	Password       string `json:"password" validate:"required,strongpass"`
	RoleID         *int64 `json:"role_id" validate:"omitempty,gt=0"`
	Worker         string `json:"worker" validate:"max=150"`
	AreaAssignment string `json:"area_assignment" validate:"max=150"`
	ChurchName     string `json:"church_name" validate:"max=150"`
}

// UpdateUserRequest is the body of PUT /admin/users/:id. It replaces the name
// and the profile text; the password and role are changed elsewhere.
type UpdateUserRequest struct {
	Name           string `json:"name" validate:"required,min=2,max=100"`
	Worker         string `json:"worker" validate:"max=150"`
	AreaAssignment string `json:"area_assignment" validate:"max=150"`
	ChurchName     string `json:"church_name" validate:"max=150"`
}

type LoginRequest struct {
	Email    string `json:"email" validate:"required,email"`
	Password string `json:"password" validate:"required"`
}
