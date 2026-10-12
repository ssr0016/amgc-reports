package config

import (
	"fmt"
	"net"
	"os"
	"strconv"
	"strings"
	"time"
)

const minAdminPasswordLen = 12

// DefaultTrustedProxyCIDR is the range Railway's edge proxies connect from.
// Production logs showed every peer_ip inside 100.64.0.0/10 (CGNAT).
const DefaultTrustedProxyCIDR = "100.64.0.0/10"

// Config holds all application configuration.
type Config struct {
	App      AppConfig
	Database DatabaseConfig
	Session  SessionConfig
	Logging  LoggingConfig
	CORS     CORSConfig
	Proxy    ProxyConfig
	Admin    AdminConfig
	Stage    Stage
}

// ProxyConfig describes which connecting peers may speak for a client.
type ProxyConfig struct {
	// TrustedCIDRs are the only peers whose X-Real-IP is believed. Loopback,
	// link-local and private ranges are not trusted unless listed here.
	TrustedCIDRs []*net.IPNet
	// UsedDefault is true when TRUSTED_PROXY_CIDRS was not set.
	UsedDefault bool
}

// AdminConfig holds the initial admin user credentials (optional).
type AdminConfig struct {
	Email    string
	Name     string
	Password string
}

// AppConfig holds general app settings.
type AppConfig struct {
	Env  string
	Port string
	URL  string
}

// DatabaseConfig holds DB connection settings.
type DatabaseConfig struct {
	Host     string
	Port     string
	User     string
	Password string
	Name     string
	URL      string
	// MigrateURL is what `api migrate` connects with. It is the owner role's
	// DSN when MIGRATE_DATABASE_URL is set, so the server can run as a
	// least-privilege role; otherwise it is URL.
	MigrateURL string
}

// SessionConfig holds session settings.
type SessionConfig struct {
	CookieName  string
	Lifetime    time.Duration
	IdleTimeout time.Duration
}

// LoggingConfig holds logging settings.
type LoggingConfig struct {
	Level  string
	Format string
}

// CORSConfig holds CORS settings.
type CORSConfig struct {
	AllowedOrigins []string
}

// Load reads configuration from environment variables.
func Load() (*Config, error) {
	cfg := &Config{
		// Port: PORT is injected by platforms such as Railway; APP_PORT wins.
		App: AppConfig{
			Env:  getEnv("APP_ENV", "local"),
			Port: getEnv("APP_PORT", getEnv("PORT", "8080")),
			URL:  getEnv("APP_URL", "http://localhost:8080"),
		},
		Database: DatabaseConfig{
			Host:     getEnv("DB_HOST", "localhost"),
			Port:     getEnv("DB_PORT", "5432"),
			User:     getEnv("DB_USER", "postgres"),
			Password: getEnv("DB_PASSWORD", ""),
			Name:     getEnv("DB_NAME", "app_db"),
			URL:      getEnv("DATABASE_URL", ""),
		},
		Session: SessionConfig{
			CookieName:  getEnv("SESSION_COOKIE_NAME", "app_session"),
			Lifetime:    getEnvDuration("SESSION_LIFETIME", 24*time.Hour),
			IdleTimeout: getEnvDuration("SESSION_IDLE_TIMEOUT", 30*time.Minute),
		},
		Logging: LoggingConfig{
			Level:  getEnv("LOG_LEVEL", "info"),
			Format: getEnv("LOG_FORMAT", "text"),
		},
		CORS: CORSConfig{
			AllowedOrigins: getEnvSlice("CORS_ALLOWED_ORIGINS", []string{"http://localhost:3000"}),
		},
	}

	trusted, usedDefault, err := trustedProxyCIDRs()
	if err != nil {
		return nil, fmt.Errorf("config validation: %w", err)
	}
	cfg.Proxy = ProxyConfig{TrustedCIDRs: trusted, UsedDefault: usedDefault}

	cfg.Admin = AdminConfig{
		Email:    strings.TrimSpace(os.Getenv("ADMIN_EMAIL")),
		Name:     getEnv("ADMIN_NAME", "Admin"),
		Password: os.Getenv("ADMIN_PASSWORD"),
	}

	cfg.Database.MigrateURL = getEnv("MIGRATE_DATABASE_URL", cfg.Database.URL)

	cfg.Stage = stageFromEnv(cfg.App.Env)

	if err := cfg.Validate(); err != nil {
		return nil, fmt.Errorf("config validation: %w", err)
	}

	return cfg, nil
}

// Validate checks required fields.
func (c *Config) Validate() error {
	if c.Database.URL == "" {
		return fmt.Errorf("DATABASE_URL is required")
	}
	if c.App.Port == "" {
		return fmt.Errorf("APP_PORT is required")
	}
	if c.Session.CookieName == "" {
		return fmt.Errorf("SESSION_COOKIE_NAME is required")
	}
	if len(c.CORS.AllowedOrigins) == 0 {
		return fmt.Errorf("CORS_ALLOWED_ORIGINS is required")
	}
	if (c.Admin.Email == "") != (c.Admin.Password == "") {
		return fmt.Errorf("ADMIN_EMAIL and ADMIN_PASSWORD must be set together")
	}
	if c.Admin.Password != "" && len(c.Admin.Password) < minAdminPasswordLen {
		return fmt.Errorf("ADMIN_PASSWORD must be at least %d characters", minAdminPasswordLen)
	}
	return nil
}

// IsProduction returns true if running in production.
func (c *Config) IsProduction() bool {
	return c.Stage == StageProd
}

// IsDevelopment returns true if running in development.
func (c *Config) IsDevelopment() bool {
	return c.Stage == StageLocal || c.Stage == StageDev
}

// IsDebug returns true if debug mode is enabled.
func (c *Config) IsDebug() bool {
	return c.Stage != StageProd
}

// trustedProxyCIDRs reads TRUSTED_PROXY_CIDRS, a comma-separated list of CIDR
// ranges. It falls back to DefaultTrustedProxyCIDR and reports that it did.
func trustedProxyCIDRs() ([]*net.IPNet, bool, error) {
	raw := getEnvSlice("TRUSTED_PROXY_CIDRS", nil)
	usedDefault := len(raw) == 0
	if usedDefault {
		raw = []string{DefaultTrustedProxyCIDR}
	}

	ranges := make([]*net.IPNet, 0, len(raw))
	for _, s := range raw {
		_, n, err := net.ParseCIDR(s)
		if err != nil {
			return nil, false, fmt.Errorf("TRUSTED_PROXY_CIDRS: invalid CIDR %q: %w", s, err)
		}
		ranges = append(ranges, n)
	}
	return ranges, usedDefault, nil
}

// ============================================================
// Helpers
// ============================================================

func getEnv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

func getEnvDuration(key string, fallback time.Duration) time.Duration {
	if v := os.Getenv(key); v != "" {
		// Try as seconds (int)
		if i, err := strconv.Atoi(v); err == nil {
			return time.Duration(i) * time.Second
		}
		// Try as duration string (e.g., "1h30m")
		if d, err := time.ParseDuration(v); err == nil {
			return d
		}
	}
	return fallback
}

func getEnvSlice(key string, fallback []string) []string {
	if v := os.Getenv(key); v != "" {
		parts := strings.Split(v, ",")
		result := make([]string, 0, len(parts))
		for _, p := range parts {
			if trimmed := strings.TrimSpace(p); trimmed != "" {
				result = append(result, trimmed)
			}
		}
		if len(result) > 0 {
			return result
		}
	}
	return fallback
}
