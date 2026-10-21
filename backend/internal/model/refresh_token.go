package model

import "time"

// RefreshToken is a stored refresh token (the token itself is never stored).
type RefreshToken struct {
	ID         int64
	UserID     int64
	FamilyID   string
	ExpiresAt  time.Time
	LastUsedAt time.Time
	RevokedAt  *time.Time
	ReplacedBy *int64
}

// NewRefreshToken is what Create stores. ExpiresAt is absolute for the family:
// a rotated token inherits it.
type NewRefreshToken struct {
	UserID    int64
	FamilyID  string
	Hash      string
	ExpiresAt time.Time
	Now       time.Time // created_at and last_used_at
	IP        string
	UserAgent string
}
