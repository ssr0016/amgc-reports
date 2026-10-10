package model

import "time"

// PasswordReset represents a password reset token.
type PasswordReset struct {
	ID        int64      `json:"id" db:"id"`
	UserID    int64      `json:"user_id" db:"user_id"`
	Token     string     `json:"token" db:"token"`
	ExpiresAt time.Time  `json:"expires_at" db:"expires_at"`
	UsedAt    *time.Time `json:"used_at,omitempty" db:"used_at"`
	CreatedAt time.Time  `json:"created_at" db:"created_at"`
}

// IsValid returns true if the token is not used and not expired.
func (p *PasswordReset) IsValid() bool {
	if p.UsedAt != nil {
		return false
	}
	return time.Now().Before(p.ExpiresAt)
}
