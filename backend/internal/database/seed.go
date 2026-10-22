package database

import (
	"context"
	"fmt"
	"log/slog"

	"golang.org/x/crypto/bcrypt"

	"github.com/ssr0016/pastoral-reports/backend/internal/accesscontrol"
)

// AdminSeed holds the credentials for the initial admin user.
// When Email or Password is empty, no admin user is seeded.
type AdminSeed struct {
	Email    string
	Name     string
	Password string
}

// SeedData inserts default roles, permissions, and (optionally) an admin user.
// Safe to call multiple times — uses ON CONFLICT DO NOTHING.
func SeedData(ctx context.Context, db *DB, log *slog.Logger, admin AdminSeed) error {
	log.Info("seeding database...")

	// ============================================================
	// 1. Roles
	// ============================================================
	roles := []struct {
		Name        string
		Description string
	}{
		{"admin", "Administrator with full access"},
		{"editor", "Editor with write access"},
		{accesscontrol.DefaultRoleName, "Default user with read-only access"},
	}

	for _, r := range roles {
		_, err := db.Pool.Exec(ctx, `
			INSERT INTO roles (name, description)
			VALUES ($1, $2)
			ON CONFLICT (name) DO NOTHING
		`, r.Name, r.Description)
		if err != nil {
			return fmt.Errorf("seed role %s: %w", r.Name, err)
		}
	}
	log.Info("roles seeded", "count", len(roles))

	// ============================================================
	// 2. Permissions
	// ============================================================
	permissions := []struct {
		Name     string
		Resource string
		Action   string
	}{
		{"users:read", "users", "read"},
		{"users:write", "users", "write"},
		{"users:delete", "users", "delete"},
		{"roles:read", "roles", "read"},
		{"roles:write", "roles", "write"},
		{"roles:delete", "roles", "delete"},
		{"permissions:read", "permissions", "read"},
		{"permissions:write", "permissions", "write"},
		{"permissions:delete", "permissions", "delete"},
		{"reports:read", "reports", "read"},
		{"reports:read_any", "reports", "read_any"},
		{"reports:write", "reports", "write"},
		{"reports:write_any", "reports", "write_any"},
		{"reports:delete", "reports", "delete"},
		{"reports:delete_any", "reports", "delete_any"},
		{"reports:approve", "reports", "approve"},
		// Admin-panel areas (37b): the admin and custom roles only, never editor or user.
		{"reports:export", "reports", "export"},
		{"tracker:read", "tracker", "read"},
		{"audit:read", "audit", "read"},
		{"users:list", "users", "list"},
	}

	for _, p := range permissions {
		_, err := db.Pool.Exec(ctx, `
			INSERT INTO permissions (name, resource, action)
			VALUES ($1, $2, $3)
			ON CONFLICT (name) DO NOTHING
		`, p.Name, p.Resource, p.Action)
		if err != nil {
			return fmt.Errorf("seed permission %s: %w", p.Name, err)
		}
	}
	log.Info("permissions seeded", "count", len(permissions))

	// ============================================================
	// 3. Role-Permission Assignments
	// ============================================================
	_, err := db.Pool.Exec(ctx, `
		INSERT INTO role_permissions (role_id, permission_id)
		SELECT r.id, p.id
		FROM roles r, permissions p
		WHERE r.name = 'admin'
		ON CONFLICT (role_id, permission_id) DO NOTHING
	`)
	if err != nil {
		return fmt.Errorf("assign admin permissions: %w", err)
	}

	_, err = db.Pool.Exec(ctx, `
		INSERT INTO role_permissions (role_id, permission_id)
		SELECT r.id, p.id
		FROM roles r, permissions p
		WHERE r.name = 'editor' AND p.name IN (
			'users:read', 'users:write',
			'reports:read', 'reports:write', 'reports:delete',
			'reports:read_any', 'reports:write_any', 'reports:delete_any')
		ON CONFLICT (role_id, permission_id) DO NOTHING
	`)
	if err != nil {
		return fmt.Errorf("assign editor permissions: %w", err)
	}

	_, err = db.Pool.Exec(ctx, `
		INSERT INTO role_permissions (role_id, permission_id)
		SELECT r.id, p.id
		FROM roles r, permissions p
		WHERE r.name = $1 AND p.name IN ('users:read', 'reports:read', 'reports:read_any', 'reports:write', 'reports:delete')
		ON CONFLICT (role_id, permission_id) DO NOTHING
	`, accesscontrol.DefaultRoleName)
	if err != nil {
		return fmt.Errorf("assign user permissions: %w", err)
	}
	log.Info("role-permission assignments seeded")

	// ============================================================
	// 4. Admin User (only when credentials are provided via env)
	// ============================================================
	if admin.Email == "" || admin.Password == "" {
		log.Warn("admin user not seeded: set ADMIN_EMAIL and ADMIN_PASSWORD to create one")
		return nil
	}

	hash, err := bcrypt.GenerateFromPassword([]byte(admin.Password), bcrypt.DefaultCost)
	if err != nil {
		return fmt.Errorf("hash admin password: %w", err)
	}

	_, err = db.Pool.Exec(ctx, `
		INSERT INTO users (email, name, password_hash, role_id, email_verified, email_verified_at)
		SELECT $1, $2, $3, r.id, TRUE, NOW()
		FROM roles r
		WHERE r.name = 'admin'
		ON CONFLICT (email) DO NOTHING
	`, admin.Email, admin.Name, string(hash))
	if err != nil {
		return fmt.Errorf("seed admin user: %w", err)
	}
	log.Info("admin user seeded", "email", admin.Email)

	return nil
}
