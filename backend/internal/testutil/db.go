package testutil

import (
	"context"
	"fmt"
	"net/url"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/modules/postgres"
	"github.com/testcontainers/testcontainers-go/wait"

	"github.com/ssr0016/pastoral-reports/backend/internal/database"
)

type TestDB struct {
	Container *postgres.PostgresContainer
	Pool      *pgxpool.Pool
	DSN       string
}

func SetupPostgres(t *testing.T) *TestDB {
	t.Helper()
	ctx := context.Background()

	container, err := postgres.Run(ctx,
		"postgres:15.3-alpine",
		postgres.WithDatabase("test_db"),
		postgres.WithUsername("test_user"),
		postgres.WithPassword("test_password"),
		testcontainers.WithWaitStrategy(
			wait.ForLog("database system is ready to accept connections").
				WithOccurrence(2).
				WithStartupTimeout(30*time.Second),
		),
	)
	if err != nil {
		t.Fatalf("failed to start postgres: %v", err)
	}

	dsn, err := container.ConnectionString(ctx, "sslmode=disable")
	if err != nil {
		t.Fatalf("failed to get conn string: %v", err)
	}

	pool, err := pgxpool.New(ctx, dsn)
	if err != nil {
		t.Fatalf("failed to connect: %v", err)
	}

	t.Cleanup(func() {
		pool.Close()
		_ = container.Terminate(ctx)
	})

	return &TestDB{Container: container, Pool: pool, DSN: dsn}
}

func (tdb *TestDB) RunMigrations(t *testing.T) {
	t.Helper()
	ctx := context.Background()

	migrations := []string{
		`CREATE TABLE IF NOT EXISTS roles (
			id BIGSERIAL PRIMARY KEY,
			name VARCHAR(50) UNIQUE NOT NULL,
			description VARCHAR(255) NOT NULL DEFAULT '',
			created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
			updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
		)`,
		`CREATE TABLE IF NOT EXISTS permissions (
			id BIGSERIAL PRIMARY KEY,
			name VARCHAR(100) UNIQUE NOT NULL,
			resource VARCHAR(50) NOT NULL,
			action VARCHAR(50) NOT NULL,
			created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
			updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
		)`,
		`CREATE TABLE IF NOT EXISTS role_permissions (
			role_id BIGINT NOT NULL REFERENCES roles(id) ON DELETE CASCADE,
			permission_id BIGINT NOT NULL REFERENCES permissions(id) ON DELETE CASCADE,
			created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
			PRIMARY KEY (role_id, permission_id)
		)`,
		`CREATE TABLE IF NOT EXISTS users (
			id BIGSERIAL PRIMARY KEY,
			email VARCHAR(255) UNIQUE NOT NULL,
			name VARCHAR(100) NOT NULL,
			password_hash VARCHAR(255) NOT NULL,
			role_id BIGINT REFERENCES roles(id) ON DELETE RESTRICT,
			email_verified BOOLEAN NOT NULL DEFAULT FALSE,
			email_verified_at TIMESTAMPTZ,
			failed_login_attempts INT NOT NULL DEFAULT 0,
			locked_until TIMESTAMPTZ,
			created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
			updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
		)`,
		`CREATE TABLE IF NOT EXISTS sessions (
			token TEXT PRIMARY KEY,
			data BYTEA NOT NULL,
			expiry TIMESTAMPTZ NOT NULL
		)`,
		`CREATE TABLE IF NOT EXISTS verification_tokens (
			id BIGSERIAL PRIMARY KEY,
			user_id BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
			token VARCHAR(255) UNIQUE NOT NULL,
			expires_at TIMESTAMPTZ NOT NULL,
			used_at TIMESTAMPTZ,
			created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
		)`,
		`CREATE TABLE IF NOT EXISTS password_resets (
			id BIGSERIAL PRIMARY KEY,
			user_id BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
			token VARCHAR(255) UNIQUE NOT NULL,
			expires_at TIMESTAMPTZ NOT NULL,
			used_at TIMESTAMPTZ,
			created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
		)`,
		`INSERT INTO roles (name, description) VALUES ('user', 'Default user role') ON CONFLICT (name) DO NOTHING`,
	}

	for _, m := range migrations {
		if _, err := tdb.Pool.Exec(ctx, m); err != nil {
			t.Fatalf("migration failed: %v\nSQL: %s", err, m)
		}
	}
}

func (tdb *TestDB) TruncateAll(t *testing.T) {
	t.Helper()
	ctx := context.Background()
	tables := []string{
		"password_resets", "verification_tokens", "role_permissions",
		"permissions", "sessions", "users", "roles",
	}
	for _, table := range tables {
		_, _ = tdb.Pool.Exec(ctx, fmt.Sprintf("TRUNCATE TABLE %s RESTART IDENTITY CASCADE", table))
	}

	// Re-seed default roles (needed for user creation)
	_, _ = tdb.Pool.Exec(ctx, `INSERT INTO roles (name, description) VALUES ('user', 'Default user role') ON CONFLICT (name) DO NOTHING`)
}

// MigrateWithGoose applies the embedded goose migrations, as `api migrate`
// does. RunMigrations above is a hand-written schema subset and does not
// create app_user.
func (tdb *TestDB) MigrateWithGoose(t *testing.T) {
	t.Helper()
	if err := database.RunMigrations(tdb.Pool); err != nil {
		t.Fatalf("goose migrations failed: %v", err)
	}
}

// AppUserPool returns a pool connected as app_user, the least-privilege role
// the migrations create. The role is NOLOGIN without a password in production
// (enabled out-of-band), so the test enables it. Call MigrateWithGoose first.
func (tdb *TestDB) AppUserPool(t *testing.T) *pgxpool.Pool {
	t.Helper()
	ctx := context.Background()

	if _, err := tdb.Pool.Exec(ctx, `ALTER ROLE app_user LOGIN PASSWORD 'app_user_pw'`); err != nil {
		t.Fatalf("app_user role is missing or unusable: %v", err)
	}

	u, err := url.Parse(tdb.DSN)
	if err != nil {
		t.Fatalf("parse dsn: %v", err)
	}
	u.User = url.UserPassword("app_user", "app_user_pw")

	pool, err := pgxpool.New(ctx, u.String())
	if err != nil {
		t.Fatalf("connect as app_user: %v", err)
	}
	t.Cleanup(pool.Close)
	return pool
}

// RoleDSN enables login for a role the migrations created and returns a DSN
// that connects as it. Call MigrateWithGoose first.
func (tdb *TestDB) RoleDSN(t *testing.T, role string) string {
	t.Helper()
	ctx := context.Background()

	// role is a fixed identifier from the tests, never user input.
	if _, err := tdb.Pool.Exec(ctx, fmt.Sprintf(`ALTER ROLE %s LOGIN PASSWORD '%s_pw'`, role, role)); err != nil {
		t.Fatalf("role %s is missing or unusable: %v", role, err)
	}

	u, err := url.Parse(tdb.DSN)
	if err != nil {
		t.Fatalf("parse dsn: %v", err)
	}
	u.User = url.UserPassword(role, role+"_pw")
	return u.String()
}

// RolePool returns a pool connected as the named role (see RoleDSN).
func (tdb *TestDB) RolePool(t *testing.T, role string) *pgxpool.Pool {
	t.Helper()

	pool, err := pgxpool.New(context.Background(), tdb.RoleDSN(t, role))
	if err != nil {
		t.Fatalf("connect as %s: %v", role, err)
	}
	t.Cleanup(pool.Close)
	return pool
}

// RemoveRole deletes the named role for the test and restores it afterwards,
// so a test can model a missing role (e.g. the default role) without leaking
// into later tests that share the database.
func (tdb *TestDB) RemoveRole(t *testing.T, name string) {
	t.Helper()
	ctx := context.Background()

	var description string
	if err := tdb.Pool.QueryRow(ctx, `DELETE FROM roles WHERE name = $1 RETURNING description`, name).Scan(&description); err != nil {
		t.Fatalf("remove role %q: %v", name, err)
	}
	t.Cleanup(func() {
		_, _ = tdb.Pool.Exec(ctx, `INSERT INTO roles (name, description) VALUES ($1, $2) ON CONFLICT (name) DO NOTHING`, name, description)
	})
}
