package database

import (
	"context"
	"fmt"
	"log/slog"

	"golang.org/x/crypto/bcrypt"
)

// SeedData inserts default roles, permissions, and admin user.
// Safe to call multiple times — uses ON CONFLICT DO NOTHING.
func SeedData(ctx context.Context, db *DB, log *slog.Logger) error {
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
		{"user", "Default user with read-only access"},
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
		WHERE r.name = 'editor' AND p.name IN ('users:read', 'users:write')
		ON CONFLICT (role_id, permission_id) DO NOTHING
	`)
	if err != nil {
		return fmt.Errorf("assign editor permissions: %w", err)
	}

	_, err = db.Pool.Exec(ctx, `
		INSERT INTO role_permissions (role_id, permission_id)
		SELECT r.id, p.id
		FROM roles r, permissions p
		WHERE r.name = 'user' AND p.name = 'users:read'
		ON CONFLICT (role_id, permission_id) DO NOTHING
	`)
	if err != nil {
		return fmt.Errorf("assign user permissions: %w", err)
	}
	log.Info("role-permission assignments seeded")

	// ============================================================
	// 4. Admin User
	// ============================================================
	adminEmail := "admin@example.com"
	adminPassword := "AdminPass123!"
	adminName := "Admin"

	hash, err := bcrypt.GenerateFromPassword([]byte(adminPassword), bcrypt.DefaultCost)
	if err != nil {
		return fmt.Errorf("hash admin password: %w", err)
	}

	_, err = db.Pool.Exec(ctx, `
		INSERT INTO users (email, name, password_hash, role_id)
		SELECT $1, $2, $3, r.id
		FROM roles r
		WHERE r.name = 'admin'
		ON CONFLICT (email) DO NOTHING
	`, adminEmail, adminName, string(hash))
	if err != nil {
		return fmt.Errorf("seed admin user: %w", err)
	}
	log.Info("admin user seeded", "email", adminEmail)

	return nil
}
