package config

import (
	"os"
	"strings"
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
		"APP_ENV", "APP_PORT", "PORT", "APP_URL",
		"DATABASE_URL", "MIGRATE_DATABASE_URL", "DB_HOST", "DB_PORT", "DB_USER", "DB_PASSWORD", "DB_NAME",
		"SESSION_COOKIE_NAME", "SESSION_LIFETIME", "SESSION_IDLE_TIMEOUT",
		"LOG_LEVEL", "LOG_FORMAT",
		"CORS_ALLOWED_ORIGINS", "TRUSTED_PROXY_CIDRS",
		"ADMIN_EMAIL", "ADMIN_PASSWORD", "ADMIN_NAME",
		"SWAGGER_ENABLED", "METRICS_USER", "METRICS_PASSWORD",
	} {
		_ = os.Unsetenv(key)
	}
}

func TestLoad_AdminNotSetByDefault(t *testing.T) {
	_ = os.Setenv("DATABASE_URL", "postgres://u:p@h:5432/db")
	defer cleanupEnv()

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() failed: %v", err)
	}
	if cfg.Admin.Email != "" || cfg.Admin.Password != "" {
		t.Errorf("Admin = %+v, want empty", cfg.Admin)
	}
}

func TestLoad_AdminFromEnv(t *testing.T) {
	_ = os.Setenv("DATABASE_URL", "postgres://u:p@h:5432/db")
	_ = os.Setenv("ADMIN_EMAIL", "boss@example.org")
	_ = os.Setenv("ADMIN_PASSWORD", "a-long-enough-pass")
	defer cleanupEnv()

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() failed: %v", err)
	}
	if cfg.Admin.Email != "boss@example.org" || cfg.Admin.Password != "a-long-enough-pass" || cfg.Admin.Name != "Admin" {
		t.Errorf("Admin = %+v", cfg.Admin)
	}
}

func TestLoad_AdminValidation(t *testing.T) {
	cases := map[string]map[string]string{
		"email without password": {"ADMIN_EMAIL": "boss@example.org"},
		"password without email": {"ADMIN_PASSWORD": "a-long-enough-pass"},
		"short password":         {"ADMIN_EMAIL": "boss@example.org", "ADMIN_PASSWORD": "short"},
	}
	for name, env := range cases {
		t.Run(name, func(t *testing.T) {
			_ = os.Setenv("DATABASE_URL", "postgres://u:p@h:5432/db")
			for k, v := range env {
				_ = os.Setenv(k, v)
			}
			defer cleanupEnv()

			if _, err := Load(); err == nil {
				t.Error("Load() succeeded, want validation error")
			}
		})
	}
}

func TestLoad_PortFallsBackToPORT(t *testing.T) {
	tests := []struct {
		name    string
		appPort string
		port    string
		want    string
	}{
		{"neither set", "", "", "8080"},
		{"PORT only", "", "3000", "3000"},
		{"APP_PORT only", "9090", "", "9090"},
		{"APP_PORT wins over PORT", "9090", "3000", "9090"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cleanupEnv()
			defer cleanupEnv()
			_ = os.Setenv("DATABASE_URL", "postgres://u:p@h:5432/db")
			if tt.appPort != "" {
				_ = os.Setenv("APP_PORT", tt.appPort)
			}
			if tt.port != "" {
				_ = os.Setenv("PORT", tt.port)
			}

			cfg, err := Load()
			if err != nil {
				t.Fatalf("Load() failed: %v", err)
			}
			if cfg.App.Port != tt.want {
				t.Errorf("App.Port = %s, want %s", cfg.App.Port, tt.want)
			}
		})
	}
}

func TestLoad_TrustedProxyCIDRs(t *testing.T) {
	tests := []struct {
		name        string
		env         string
		want        []string
		wantDefault bool
	}{
		{"unset uses the Railway default", "", []string{"100.64.0.0/10"}, true},
		{"blank entries use the default", " , ", []string{"100.64.0.0/10"}, true},
		{"single range", "10.0.0.0/8", []string{"10.0.0.0/8"}, false},
		{"several ranges with spaces", "10.0.0.0/8, fd00::/8", []string{"10.0.0.0/8", "fd00::/8"}, false},
		{"host bits are masked", "100.64.3.4/10", []string{"100.64.0.0/10"}, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cleanupEnv()
			defer cleanupEnv()
			_ = os.Setenv("DATABASE_URL", "postgres://u:p@h:5432/db")
			if tt.env != "" {
				_ = os.Setenv("TRUSTED_PROXY_CIDRS", tt.env)
			}

			cfg, err := Load()
			if err != nil {
				t.Fatalf("Load() failed: %v", err)
			}

			got := make([]string, 0, len(cfg.Proxy.TrustedCIDRs))
			for _, n := range cfg.Proxy.TrustedCIDRs {
				got = append(got, n.String())
			}
			if len(got) != len(tt.want) {
				t.Fatalf("TrustedCIDRs = %v, want %v", got, tt.want)
			}
			for i := range got {
				if got[i] != tt.want[i] {
					t.Errorf("TrustedCIDRs[%d] = %s, want %s", i, got[i], tt.want[i])
				}
			}
			if cfg.Proxy.UsedDefault != tt.wantDefault {
				t.Errorf("UsedDefault = %v, want %v", cfg.Proxy.UsedDefault, tt.wantDefault)
			}
		})
	}
}

func TestLoad_TrustedProxyCIDRsRejectsInvalidEntries(t *testing.T) {
	for _, env := range []string{"not-a-cidr", "10.0.0.0/8,300.1.1.1/8", "10.0.0.1", "10.0.0.0/33"} {
		t.Run(env, func(t *testing.T) {
			cleanupEnv()
			defer cleanupEnv()
			_ = os.Setenv("DATABASE_URL", "postgres://u:p@h:5432/db")
			_ = os.Setenv("TRUSTED_PROXY_CIDRS", env)

			_, err := Load()
			if err == nil {
				t.Fatal("Load() succeeded, want an error")
			}
			if !strings.Contains(err.Error(), "TRUSTED_PROXY_CIDRS") {
				t.Errorf("error %q does not name TRUSTED_PROXY_CIDRS", err)
			}
		})
	}
}

func TestLoad_MigrateURL_FallsBackToDatabaseURL(t *testing.T) {
	_ = os.Setenv("DATABASE_URL", "postgres://app:p@h:5432/db")
	defer cleanupEnv()

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() failed: %v", err)
	}
	if cfg.Database.MigrateURL != cfg.Database.URL {
		t.Errorf("MigrateURL = %q, want fallback to %q", cfg.Database.MigrateURL, cfg.Database.URL)
	}
}

func TestLoad_MigrateURL_UsesMigrateDatabaseURL(t *testing.T) {
	_ = os.Setenv("DATABASE_URL", "postgres://app:p@h:5432/db")
	_ = os.Setenv("MIGRATE_DATABASE_URL", "postgres://owner:p@h:5432/db")
	defer cleanupEnv()

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() failed: %v", err)
	}
	if cfg.Database.MigrateURL != "postgres://owner:p@h:5432/db" {
		t.Errorf("MigrateURL = %q, want the MIGRATE_DATABASE_URL value", cfg.Database.MigrateURL)
	}
	if cfg.Database.URL != "postgres://app:p@h:5432/db" {
		t.Errorf("URL = %q, MIGRATE_DATABASE_URL must not change the app URL", cfg.Database.URL)
	}
}

// loadWithEnv runs Load with DATABASE_URL and the given variables set.
func loadWithEnv(t *testing.T, env map[string]string) (*Config, error) {
	t.Helper()
	t.Cleanup(cleanupEnv)
	t.Setenv("DATABASE_URL", "postgres://u:p@h:5432/db")
	for k, v := range env {
		t.Setenv(k, v)
	}
	return Load()
}

// Swagger is on outside production and off in production, so a deploy that
// forgets to configure it does not publish the API map.
func TestLoad_ObservabilityDefaultsByStage(t *testing.T) {
	tests := []struct {
		appEnv string
		want   bool
	}{
		{"production", false},
		{"prod", false},
		{"development", true},
		{"dev", true},
		{"", true},
	}
	for _, tt := range tests {
		t.Run("APP_ENV="+tt.appEnv, func(t *testing.T) {
			env := map[string]string{}
			if tt.appEnv != "" {
				env["APP_ENV"] = tt.appEnv
			}
			cfg, err := loadWithEnv(t, env)
			if err != nil {
				t.Fatalf("Load() failed: %v", err)
			}
			if got := cfg.Observability.SwaggerEnabled; got != tt.want {
				t.Errorf("SwaggerEnabled = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestLoad_SwaggerEnabledOverride(t *testing.T) {
	tests := []struct {
		name  string
		env   map[string]string
		want  bool
		stage string
	}{
		{"production, explicitly on", map[string]string{"APP_ENV": "production", "SWAGGER_ENABLED": "true"}, true, "prod"},
		{"development, explicitly off", map[string]string{"APP_ENV": "development", "SWAGGER_ENABLED": "false"}, false, "dev"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg, err := loadWithEnv(t, tt.env)
			if err != nil {
				t.Fatalf("Load() failed: %v", err)
			}
			if got := cfg.Observability.SwaggerEnabled; got != tt.want {
				t.Errorf("SwaggerEnabled = %v, want %v", got, tt.want)
			}
		})
	}
}

// A typo must stop the server, not silently pick a default.
func TestLoad_SwaggerEnabledInvalidValue(t *testing.T) {
	_, err := loadWithEnv(t, map[string]string{"SWAGGER_ENABLED": "maybe"})
	if err == nil {
		t.Fatal("Load() = nil error, want one for SWAGGER_ENABLED=maybe")
	}
	if !strings.Contains(err.Error(), "SWAGGER_ENABLED") {
		t.Errorf("error = %q, want it to name SWAGGER_ENABLED", err)
	}
}

func TestLoad_MetricsCredentialsValidation(t *testing.T) {
	const strong = "a-long-enough-pass"
	bad := map[string]map[string]string{
		"user without password": {"METRICS_USER": "scraper"},
		"password without user": {"METRICS_PASSWORD": strong},
		"short password":        {"METRICS_USER": "scraper", "METRICS_PASSWORD": "short"},
	}
	for name, env := range bad {
		t.Run(name, func(t *testing.T) {
			if _, err := loadWithEnv(t, env); err == nil {
				t.Error("Load() = nil error, want a validation error")
			}
		})
	}

	t.Run("both set", func(t *testing.T) {
		cfg, err := loadWithEnv(t, map[string]string{"METRICS_USER": "scraper", "METRICS_PASSWORD": strong})
		if err != nil {
			t.Fatalf("Load() failed: %v", err)
		}
		if cfg.Observability.MetricsUser != "scraper" || cfg.Observability.MetricsPassword != strong {
			t.Errorf("Observability = %+v, want the METRICS_* values", cfg.Observability)
		}
	})
}
