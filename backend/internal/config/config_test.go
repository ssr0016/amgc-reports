package config

import (
	"os"
	"strings"
	"testing"
	"time"
)

func TestValidateAuth(t *testing.T) {
	good := AuthConfig{JWTSecret: "0123456789abcdef0123456789abcdef", AccessTTL: time.Minute, RefreshTTL: time.Hour, RefreshIdleTTL: time.Minute}
	tests := []struct {
		name    string
		mutate  func(*AuthConfig)
		wantErr string
	}{
		{"valid", func(*AuthConfig) {}, ""},
		{"missing secret", func(a *AuthConfig) { a.JWTSecret = "" }, "JWT_SECRET"},
		{"short secret", func(a *AuthConfig) { a.JWTSecret = "too-short" }, "JWT_SECRET"},
		{"zero access ttl", func(a *AuthConfig) { a.AccessTTL = 0 }, "TTL"},
		{"negative refresh ttl", func(a *AuthConfig) { a.RefreshTTL = -time.Hour }, "TTL"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			a := good
			tt.mutate(&a)
			err := (&Config{Auth: a}).ValidateAuth()
			if tt.wantErr == "" {
				if err != nil {
					t.Fatalf("ValidateAuth() = %v, want nil", err)
				}
				return
			}
			if err == nil || !strings.Contains(err.Error(), tt.wantErr) {
				t.Fatalf("ValidateAuth() = %v, want an error naming %s", err, tt.wantErr)
			}
		})
	}
}

// api migrate loads the config before the secret exists; it must still work.
func TestLoad_DoesNotRequireJWTSecret(t *testing.T) {
	_ = os.Setenv("DATABASE_URL", "postgres://u:p@h:5432/db")
	_ = os.Setenv("CORS_ALLOWED_ORIGINS", "http://localhost:3000")
	defer cleanupEnv()
	_ = os.Unsetenv("JWT_SECRET")
	if _, err := Load(); err != nil {
		t.Fatalf("Load() without JWT_SECRET: %v", err)
	}
}

func TestLoad_WithAllEnvVars(t *testing.T) {
	// Set env vars
	_ = os.Setenv("APP_ENV", "production")
	_ = os.Setenv("APP_PORT", "9090")
	_ = os.Setenv("APP_URL", "https://api.example.com")
	_ = os.Setenv("DATABASE_URL", "postgres://user:pass@host:5432/db")
	_ = os.Setenv("REFRESH_TTL", "3600")
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

	// Auth
	if cfg.Auth.RefreshTTL != 3600*time.Second {
		t.Errorf("Auth.RefreshTTL = %v, want 1h", cfg.Auth.RefreshTTL)
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
	if cfg.Auth.AccessTTL != 10*time.Minute || cfg.Auth.RefreshTTL != 24*time.Hour || cfg.Auth.RefreshIdleTTL != 30*time.Minute {
		t.Errorf("Auth TTL defaults = %v/%v/%v, want 10m/24h/30m", cfg.Auth.AccessTTL, cfg.Auth.RefreshTTL, cfg.Auth.RefreshIdleTTL)
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
		"JWT_SECRET", "JWT_ISSUER", "JWT_AUDIENCE", "JWT_ACCESS_TTL", "REFRESH_TTL", "REFRESH_IDLE_TTL", "COOKIE_DOMAIN",
		"LOG_LEVEL", "LOG_FORMAT",
		"CORS_ALLOWED_ORIGINS", "TRUSTED_PROXY_CIDRS",
		"ADMIN_EMAIL", "ADMIN_PASSWORD", "ADMIN_NAME",
		"SWAGGER_ENABLED", "METRICS_USER", "METRICS_PASSWORD",
		"REPORTS_WRITE_RATE_LIMIT", "REPORTS_WRITE_BURST", "REPORTS_READ_RATE_LIMIT", "REPORTS_READ_BURST",
		"R2_ACCOUNT_ID", "R2_ACCESS_KEY_ID", "R2_SECRET_ACCESS_KEY", "R2_BUCKET", "R2_PUBLIC_URL",
	} {
		_ = os.Unsetenv(key)
	}
}

func TestLoad_StoragePublicURLPreservesFullValue(t *testing.T) {
	const publicURL = "https://pub-34d82063b0224b23bf6873227edb3e55.r2.dev"
	cfg, err := loadWithEnv(t, map[string]string{
		"CORS_ALLOWED_ORIGINS": "http://localhost:3000",
		"R2_ACCOUNT_ID":        "account", "R2_ACCESS_KEY_ID": "key", "R2_SECRET_ACCESS_KEY": "secret",
		"R2_BUCKET": "avatars", "R2_PUBLIC_URL": publicURL + "///",
	})
	if err != nil {
		t.Fatalf("Load() failed: %v", err)
	}
	if cfg.Storage.PublicURL != publicURL {
		t.Fatalf("Storage.PublicURL = %q, want %q", cfg.Storage.PublicURL, publicURL)
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
		{"unset uses the default", "", []string{"0.0.0.0/0"}, true},
		{"blank entries use the default", " , ", []string{"0.0.0.0/0"}, true},
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
	for _, env := range []string{"not-a-cidr", "10.0.0.0/8,300.1.1.1/8", "10.0.0.1", "10.0.0.0/33", "fd00::/129", "fd00::", "10.0.0.0/8/1", "10.0.0.0/-1"} {
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

func TestLoad_ReportsLimits(t *testing.T) {
	defaults := ReportsConfig{WriteRatePerMinute: 60, WriteBurst: 10, ReadRatePerMinute: 60, ReadBurst: 20}
	tests := []struct {
		name    string
		env     map[string]string
		want    ReportsConfig
		wantErr bool
	}{
		{"defaults", nil, defaults, false},
		{"custom write", map[string]string{"REPORTS_WRITE_RATE_LIMIT": "120", "REPORTS_WRITE_BURST": "20"},
			ReportsConfig{WriteRatePerMinute: 120, WriteBurst: 20, ReadRatePerMinute: 60, ReadBurst: 20}, false},
		{"custom read", map[string]string{"REPORTS_READ_RATE_LIMIT": "6000", "REPORTS_READ_BURST": "1000"},
			ReportsConfig{WriteRatePerMinute: 60, WriteBurst: 10, ReadRatePerMinute: 6000, ReadBurst: 1000}, false},
		{"zero write rate", map[string]string{"REPORTS_WRITE_RATE_LIMIT": "0"}, ReportsConfig{}, true},
		{"negative write burst", map[string]string{"REPORTS_WRITE_BURST": "-1"}, ReportsConfig{}, true},
		{"write not a number", map[string]string{"REPORTS_WRITE_RATE_LIMIT": "fast"}, ReportsConfig{}, true},
		{"zero read rate", map[string]string{"REPORTS_READ_RATE_LIMIT": "0"}, ReportsConfig{}, true},
		{"read burst not a number", map[string]string{"REPORTS_READ_BURST": "many"}, ReportsConfig{}, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_ = os.Setenv("DATABASE_URL", "postgres://u:p@h:5432/db")
			for k, v := range tt.env {
				_ = os.Setenv(k, v)
			}
			defer cleanupEnv()

			cfg, err := Load()
			if tt.wantErr {
				if err == nil {
					t.Fatal("Load() succeeded, want error")
				}
				return
			}
			if err != nil {
				t.Fatalf("Load() failed: %v", err)
			}
			if cfg.Reports != tt.want {
				t.Errorf("Reports = %+v, want %+v", cfg.Reports, tt.want)
			}
		})
	}
}

func TestLoad_DatabasePoolDefaultsAndOverrides(t *testing.T) {
	_ = os.Setenv("DATABASE_URL", "postgres://u:p@h:5432/db")
	_ = os.Setenv("CORS_ALLOWED_ORIGINS", "http://localhost:3000")
	defer cleanupEnv()
	defer func() {
		for _, k := range []string{"DB_MAX_CONNS", "DB_MIN_CONNS", "DB_CONN_MAX_LIFETIME", "DB_CONN_MAX_IDLE_TIME"} {
			_ = os.Unsetenv(k)
		}
	}()

	cfg, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	d := cfg.Database
	if d.MaxConns != 20 || d.MinConns != 0 || d.ConnMaxLifetime != time.Hour || d.ConnMaxIdleTime != 30*time.Minute {
		t.Errorf("defaults = %d/%d/%v/%v, want 20/0/1h/30m", d.MaxConns, d.MinConns, d.ConnMaxLifetime, d.ConnMaxIdleTime)
	}

	_ = os.Setenv("DB_MAX_CONNS", "8")
	_ = os.Setenv("DB_MIN_CONNS", "2")
	_ = os.Setenv("DB_CONN_MAX_IDLE_TIME", "300")
	cfg, err = Load()
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Database.MaxConns != 8 || cfg.Database.MinConns != 2 || cfg.Database.ConnMaxIdleTime != 5*time.Minute {
		t.Errorf("overrides = %+v", cfg.Database)
	}
}

func TestLoad_DatabasePoolRejectsBadValues(t *testing.T) {
	for name, env := range map[string][2]string{
		"zero max":          {"DB_MAX_CONNS", "0"},
		"text max":          {"DB_MAX_CONNS", "lots"},
		"min above max":     {"DB_MIN_CONNS", "99"},
		"negative min text": {"DB_MIN_CONNS", "-1"},
	} {
		t.Run(name, func(t *testing.T) {
			_ = os.Setenv("DATABASE_URL", "postgres://u:p@h:5432/db")
			_ = os.Setenv("CORS_ALLOWED_ORIGINS", "http://localhost:3000")
			_ = os.Setenv(env[0], env[1])
			defer cleanupEnv()
			defer func() { _ = os.Unsetenv(env[0]) }()
			if _, err := Load(); err == nil {
				t.Fatalf("Load() with %s=%s succeeded, want an error", env[0], env[1])
			}
		})
	}
}

func TestValidateProxy(t *testing.T) {
	tests := []struct {
		name    string
		stage   Stage
		env     string // TRUSTED_PROXY_CIDRS; "" leaves it unset
		wantErr bool
	}{
		{"production with a range", StageProd, "100.64.0.0/10", false},
		{"production with several ranges", StageProd, "100.64.0.0/10, fd00::/8", false},
		{"production unset", StageProd, "", true},
		{"production blank entries", StageProd, " , ", true},
		{"production trusts every IPv4 peer", StageProd, "0.0.0.0/0", true},
		{"production trusts every IPv6 peer", StageProd, "::/0", true},
		{"production /0 hidden among ranges", StageProd, "100.64.0.0/10,0.0.0.0/0", true},
		{"production /0 written with host bits", StageProd, "10.1.2.3/0", true},
		{"local unset keeps the default", StageLocal, "", false},
		{"dev /0 allowed", StageDev, "0.0.0.0/0", false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cleanupEnv()
			defer cleanupEnv()
			if tt.env != "" {
				_ = os.Setenv("TRUSTED_PROXY_CIDRS", tt.env)
			}
			cfg := &Config{Stage: tt.stage}
			if err := cfg.proxyConfig(); err != nil {
				t.Fatalf("proxyConfig() = %v", err)
			}

			err := cfg.ValidateProxy()
			if tt.wantErr {
				if err == nil || !strings.Contains(err.Error(), "TRUSTED_PROXY_CIDRS") {
					t.Fatalf("ValidateProxy() = %v, want an error naming TRUSTED_PROXY_CIDRS", err)
				}
				return
			}
			if err != nil {
				t.Fatalf("ValidateProxy() = %v, want nil", err)
			}
		})
	}
}
