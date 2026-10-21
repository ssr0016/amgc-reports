package config

import (
	"fmt"
	"os"
	"strings"
	"time"
)

const minAdminPasswordLen = 12

// minJWTSecretLen is the shortest HS256 secret accepted: 32 bytes, the size of the hash.
const minJWTSecretLen = 32

// AuthConfig holds the JWT sign-in settings (D-AR1).
type AuthConfig struct {
	JWTSecret   string
	JWTIssuer   string
	JWTAudience string
	// AccessTTL is the life of an access token; RefreshTTL is the absolute life of
	// a sign-in, and RefreshIdleTTL how long it may go without a refresh.
	AccessTTL      time.Duration
	RefreshTTL     time.Duration
	RefreshIdleTTL time.Duration
	// CookieDomain is empty (host-only cookies) until D-F1 sets .amgreports.com.
	CookieDomain string
}

// AdminConfig holds the initial admin user credentials (optional).
type AdminConfig struct {
	Email    string
	Name     string
	Password string
}

func (cfg *Config) authConfig() {
	cfg.Auth = AuthConfig{
		JWTSecret:      os.Getenv("JWT_SECRET"),
		JWTIssuer:      getEnv("JWT_ISSUER", "amgreports"),
		JWTAudience:    getEnv("JWT_AUDIENCE", "amgreports-web"),
		AccessTTL:      getEnvDuration("JWT_ACCESS_TTL", 10*time.Minute),
		RefreshTTL:     getEnvDuration("REFRESH_TTL", 24*time.Hour),
		RefreshIdleTTL: getEnvDuration("REFRESH_IDLE_TTL", 30*time.Minute),
		CookieDomain:   strings.TrimSpace(os.Getenv("COOKIE_DOMAIN")),
	}
}

func (cfg *Config) adminConfig() {
	cfg.Admin = AdminConfig{
		Email:    strings.TrimSpace(os.Getenv("ADMIN_EMAIL")),
		Name:     getEnv("ADMIN_NAME", "Admin"),
		Password: os.Getenv("ADMIN_PASSWORD"),
	}
}

// ValidateAuth checks the JWT settings. The server calls it at startup and fails
// closed: a missing or short secret would let anyone sign a token. It is not part
// of Validate because `api migrate`, which runs before the new secret may exist,
// does not sign anything.
func (c *Config) ValidateAuth() error {
	if len(c.Auth.JWTSecret) < minJWTSecretLen {
		return fmt.Errorf("JWT_SECRET is required and must be at least %d bytes", minJWTSecretLen)
	}
	if c.Auth.AccessTTL <= 0 || c.Auth.RefreshTTL <= 0 || c.Auth.RefreshIdleTTL <= 0 {
		return fmt.Errorf("JWT_ACCESS_TTL, REFRESH_TTL and REFRESH_IDLE_TTL must be positive")
	}
	return nil
}
