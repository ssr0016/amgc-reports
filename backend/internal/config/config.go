package config

import (
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"
)

// Config holds all application configuration.
type Config struct {
	App      AppConfig
	Database DatabaseConfig
	Session  SessionConfig
	Logging  LoggingConfig
	CORS     CORSConfig
	Stage    Stage
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
		App: AppConfig{
			Env:  getEnv("APP_ENV", "local"),
			Port: getEnv("APP_PORT", "8080"),
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
