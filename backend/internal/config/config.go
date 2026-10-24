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
	Auth     AuthConfig
	Logging  LoggingConfig
	CORS     CORSConfig
	Proxy    ProxyConfig
	Reports  ReportsConfig
	Admin    AdminConfig
	Storage  StorageConfig
	Stage    Stage

	Observability ObservabilityConfig
}

// Load reads configuration from environment variables.
func Load() (*Config, error) {
	cfg := &Config{}
	cfg.appConfig()
	cfg.databaseConfig()
	cfg.authConfig()
	cfg.loggingConfig()
	cfg.corsConfig()

	if err := cfg.proxyConfig(); err != nil {
		return nil, fmt.Errorf("config validation: %w", err)
	}

	if err := cfg.reportsConfig(); err != nil {
		return nil, fmt.Errorf("config validation: %w", err)
	}

	cfg.adminConfig()

	if err := cfg.storageConfig(); err != nil {
		return nil, fmt.Errorf("config validation: %w", err)
	}

	if err := cfg.databasePoolConfig(); err != nil {
		return nil, fmt.Errorf("config validation: %w", err)
	}

	cfg.Stage = stageFromEnv(cfg.App.Env)

	if err := cfg.observabilityConfig(); err != nil {
		return nil, fmt.Errorf("config validation: %w", err)
	}

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
	if c.Database.MaxConns < 1 {
		return fmt.Errorf("DB_MAX_CONNS must be at least 1")
	}
	if c.Database.MinConns < 0 || c.Database.MinConns > c.Database.MaxConns {
		return fmt.Errorf("DB_MIN_CONNS must be between 0 and DB_MAX_CONNS")
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
	if (c.Observability.MetricsUser == "") != (c.Observability.MetricsPassword == "") {
		return fmt.Errorf("METRICS_USER and METRICS_PASSWORD must be set together")
	}
	if p := c.Observability.MetricsPassword; p != "" && len(p) < minMetricsPasswordLen {
		return fmt.Errorf("METRICS_PASSWORD must be at least %d characters", minMetricsPasswordLen)
	}
	return nil
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

// getEnvBool reads a boolean. An unset or empty variable gives fallback; a
// value that is not a boolean is an error, so a typo cannot pick a default.
func getEnvBool(key string, fallback bool) (bool, error) {
	v := os.Getenv(key)
	if v == "" {
		return fallback, nil
	}
	b, err := strconv.ParseBool(v)
	if err != nil {
		return false, fmt.Errorf("%s must be true or false, got %q", key, v)
	}
	return b, nil
}

// getEnvInt reads a positive integer. An unset or empty variable gives
// fallback; anything else that is not a positive integer is an error, so a
// typo or a 0 (which would block every write) cannot slip through.
func getEnvInt(key string, fallback int) (int, error) {
	v := os.Getenv(key)
	if v == "" {
		return fallback, nil
	}
	n, err := strconv.Atoi(v)
	if err != nil || n < 1 {
		return 0, fmt.Errorf("%s must be a positive integer, got %q", key, v)
	}
	return n, nil
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
