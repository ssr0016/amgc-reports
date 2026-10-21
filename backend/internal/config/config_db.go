package config

import "time"

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

	// Pool bounds the server's connections. The 20 default suits a managed
	// Postgres with a modest connection limit (Neon's direct endpoint); zero
	// keeps the driver's default. ConnMaxLifetime and ConnMaxIdleTime take
	// seconds or a Go duration.
	MaxConns        int
	MinConns        int
	ConnMaxLifetime time.Duration
	ConnMaxIdleTime time.Duration
}

func (cfg *Config) databaseConfig() {
	cfg.Database = DatabaseConfig{
		Host:     getEnv("DB_HOST", "localhost"),
		Port:     getEnv("DB_PORT", "5432"),
		User:     getEnv("DB_USER", "postgres"),
		Password: getEnv("DB_PASSWORD", ""),
		Name:     getEnv("DB_NAME", "app_db"),
		URL:      getEnv("DATABASE_URL", ""),
	}
}

func (cfg *Config) databasePoolConfig() error {
	cfg.Database.MigrateURL = getEnv("MIGRATE_DATABASE_URL", cfg.Database.URL)

	var err error
	if cfg.Database.MaxConns, err = getEnvInt("DB_MAX_CONNS", 20); err != nil {
		return err
	}
	if cfg.Database.MinConns, err = getEnvInt("DB_MIN_CONNS", 0); err != nil {
		return err
	}
	cfg.Database.ConnMaxLifetime = getEnvDuration("DB_CONN_MAX_LIFETIME", time.Hour)
	cfg.Database.ConnMaxIdleTime = getEnvDuration("DB_CONN_MAX_IDLE_TIME", 30*time.Minute)
	return nil
}
