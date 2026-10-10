package config

import (
	"os"
	"testing"
	"time"
)

func TestLoad_WithAllEnvVars(t *testing.T) {
	// Set env vars
	_ = os.Setenv("APP_ENV", "production")
	_ = os.Setenv("APP_PORT", "9090")
	_ = os.Setenv("APP_URL", "https://api.example.com")
	_ = os.Setenv("DATABASE_URL", "postgres://user:pass@host:5432/db")
	_ = os.Setenv("SESSION_COOKIE_NAME", "my_session")
	_ = os.Setenv("SESSION_LIFETIME", "3600")
	_ = os.Setenv("LOG_LEVEL", "warn")
	_ = os.Setenv("LOG_FORMAT", "json")
	_ = os.Setenv("CORS_ALLOWED_ORIGINS", "https://app.example.com,https://admin.example.com")
	defer cleanupEnv()

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() failed: %v", err)
	}

	// App
	if cfg.App.Env != "production" {
		t.Errorf("App.Env = %s, want production", cfg.App.Env)
	}
	if cfg.App.Port != "9090" {
		t.Errorf("App.Port = %s, want 9090", cfg.App.Port)
	}
	if cfg.App.URL != "https://api.example.com" {
		t.Errorf("App.URL = %s, want https://api.example.com", cfg.App.URL)
	}

	// Database
	if cfg.Database.URL != "postgres://user:pass@host:5432/db" {
		t.Errorf("Database.URL = %s, want postgres://user:pass@host:5432/db", cfg.Database.URL)
	}

	// Session
	if cfg.Session.CookieName != "my_session" {
		t.Errorf("Session.CookieName = %s, want my_session", cfg.Session.CookieName)
	}
	if cfg.Session.Lifetime != 3600*time.Second {
		t.Errorf("Session.Lifetime = %v, want 1h", cfg.Session.Lifetime)
	}

	// Logging
	if cfg.Logging.Level != "warn" {
		t.Errorf("Logging.Level = %s, want warn", cfg.Logging.Level)
	}
	if cfg.Logging.Format != "json" {
		t.Errorf("Logging.Format = %s, want json", cfg.Logging.Format)
	}

	// CORS
	if len(cfg.CORS.AllowedOrigins) != 2 {
		t.Errorf("CORS.AllowedOrigins length = %d, want 2", len(cfg.CORS.AllowedOrigins))
	}

	// Stage
	if cfg.Stage != StageProd {
		t.Errorf("Stage = %s, want prod", cfg.Stage)
	}
	if !cfg.IsProduction() {
		t.Error("IsProduction() = false, want true")
	}
}

func TestLoad_WithDefaults(t *testing.T) {
	cleanupEnv()
	_ = os.Setenv("DATABASE_URL", "postgres://localhost/test")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() failed: %v", err)
	}

	if cfg.App.Env != "local" {
		t.Errorf("App.Env = %s, want local", cfg.App.Env)
	}
	if cfg.App.Port != "8080" {
		t.Errorf("App.Port = %s, want 8080", cfg.App.Port)
	}
	if cfg.Session.CookieName != "app_session" {
		t.Errorf("Session.CookieName = %s, want app_session", cfg.Session.CookieName)
	}
	if cfg.Logging.Level != "info" {
		t.Errorf("Logging.Level = %s, want info", cfg.Logging.Level)
	}
	if cfg.Logging.Format != "text" {
		t.Errorf("Logging.Format = %s, want text", cfg.Logging.Format)
	}
}

func TestLoad_MissingDatabaseURL(t *testing.T) {
	cleanupEnv()

	_, err := Load()
	if err == nil {
		t.Fatal("Load() expected error, got nil")
	}
}

func TestStage_FromEnv(t *testing.T) {
	tests := []struct {
		env  string
		want Stage
	}{
		{"local", StageLocal},
		{"development", StageDev},
		{"dev", StageDev},
		{"prod", StageProd},
		{"production", StageProd},
		{"unknown", StageLocal},
		{"", StageLocal},
	}

	for _, tt := range tests {
		t.Run(tt.env, func(t *testing.T) {
			got := stageFromEnv(tt.env)
			if got != tt.want {
				t.Errorf("stageFromEnv(%q) = %s, want %s", tt.env, got, tt.want)
			}
		})
	}
}

func TestConfig_IsMethods(t *testing.T) {
	tests := []struct {
		stage     Stage
		wantProd  bool
		wantDev   bool
		wantDebug bool
	}{
		{StageLocal, false, true, true},
		{StageDev, false, true, true},
		{StageProd, true, false, false},
	}

	for _, tt := range tests {
		t.Run(string(tt.stage), func(t *testing.T) {
			cfg := &Config{Stage: tt.stage}
			if got := cfg.IsProduction(); got != tt.wantProd {
				t.Errorf("IsProduction() = %v, want %v", got, tt.wantProd)
			}
			if got := cfg.IsDevelopment(); got != tt.wantDev {
				t.Errorf("IsDevelopment() = %v, want %v", got, tt.wantDev)
			}
			if got := cfg.IsDebug(); got != tt.wantDebug {
				t.Errorf("IsDebug() = %v, want %v", got, tt.wantDebug)
			}
		})
	}
}

// cleanupEnv removes all env vars used by config.
func cleanupEnv() {
	for _, key := range []string{
		"APP_ENV", "APP_PORT", "APP_URL",
		"DATABASE_URL", "DB_HOST", "DB_PORT", "DB_USER", "DB_PASSWORD", "DB_NAME",
		"SESSION_COOKIE_NAME", "SESSION_LIFETIME", "SESSION_IDLE_TIMEOUT",
		"LOG_LEVEL", "LOG_FORMAT",
		"CORS_ALLOWED_ORIGINS",
	} {
		_ = os.Unsetenv(key)
	}
}
