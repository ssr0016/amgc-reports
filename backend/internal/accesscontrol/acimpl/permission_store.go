package acimpl

import (
	"context"
	"errors"
	"fmt"

	sq "github.com/Masterminds/squirrel"
	"github.com/jackc/pgx/v5"

	"github.com/ssr0016/pastoral-reports/backend/internal/accesscontrol"
	"github.com/ssr0016/pastoral-reports/backend/internal/database"
)

type permissionStore struct{ db *database.DB }

// NewPermissionStore returns the accesscontrol.PermissionStore over db.
func NewPermissionStore(db *database.DB) accesscontrol.PermissionStore {
	return &permissionStore{db: db}
}

const permissionColumns = "id, name, resource, action, created_at, updated_at"

// scanPermission scans a single permission from a row.
func scanPermission(row pgx.Row) (*accesscontrol.Permission, error) {
	var p accesscontrol.Permission
	err := row.Scan(&p.ID, &p.Name, &p.Resource, &p.Action, &p.CreatedAt, &p.UpdatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if database.IsUniqueViolation(err) {
		return nil, fmt.Errorf("scan permission: %w: %w", database.ErrUniqueViolation, err)
	}
	if err != nil {
		return nil, fmt.Errorf("scan permission: %w", err)
	}
	return &p, nil
}

// findByColumn is a helper for fetching a single permission by any column.
func (r *permissionStore) findByColumn(ctx context.Context, column string, value interface{}) (*accesscontrol.Permission, error) {
	query, args, err := r.db.Builder.
		Select(permissionColumns).
		From("permissions").
		Where(sq.Eq{column: value}).
		ToSql()
	if err != nil {
		return nil, fmt.Errorf("build query: %w", err)
	}
	return scanPermission(r.db.Pool.QueryRow(ctx, query, args...))
}

func (r *permissionStore) Create(ctx context.Context, name, resource, action string) (*accesscontrol.Permission, error) {
	query, args, err := r.db.Builder.
		Insert("permissions").
		Columns("name", "resource", "action").
		Values(name, resource, action).
		Suffix("RETURNING " + permissionColumns).
		ToSql()
	if err != nil {
		return nil, fmt.Errorf("build query: %w", err)
	}
	return scanPermission(r.db.Pool.QueryRow(ctx, query, args...))
}

func (r *permissionStore) GetByID(ctx context.Context, id int64) (*accesscontrol.Permission, error) {
	return r.findByColumn(ctx, "id", id)
}

func (r *permissionStore) GetByName(ctx context.Context, name string) (*accesscontrol.Permission, error) {
	return r.findByColumn(ctx, "name", name)
}

func (r *permissionStore) List(ctx context.Context) ([]accesscontrol.Permission, error) {
	query, args, err := r.db.Builder.
		Select(permissionColumns).
		From("permissions").
		OrderBy("resource ASC, action ASC").
		ToSql()
	if err != nil {
		return nil, fmt.Errorf("build query: %w", err)
	}
	return r.scanPermissions(ctx, query, args...)
}

func (r *permissionStore) ListByResource(ctx context.Context, resource string) ([]accesscontrol.Permission, error) {
	query, args, err := r.db.Builder.
		Select(permissionColumns).
		From("permissions").
		Where(sq.Eq{"resource": resource}).
		OrderBy("action ASC").
		ToSql()
	if err != nil {
		return nil, fmt.Errorf("build query: %w", err)
	}
	return r.scanPermissions(ctx, query, args...)
}

func (r *permissionStore) Delete(ctx context.Context, id int64) error {
	return deleteByID(ctx, r.db, "permissions", id)
}

func (r *permissionStore) scanPermissions(ctx context.Context, query string, args ...interface{}) ([]accesscontrol.Permission, error) {
	rows, err := r.db.Pool.Query(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("query permissions: %w", err)
	}
	defer rows.Close()

	var perms []accesscontrol.Permission
	for rows.Next() {
		var p accesscontrol.Permission
		if err := rows.Scan(&p.ID, &p.Name, &p.Resource, &p.Action, &p.CreatedAt, &p.UpdatedAt); err != nil {
			return nil, fmt.Errorf("scan permission: %w", err)
		}
		perms = append(perms, p)
	}
	return perms, rows.Err()
}

// ListWithPagination returns permissions with pagination and total count.
func (r *permissionStore) ListWithPagination(ctx context.Context, resource string, page, limit int) ([]accesscontrol.Permission, int64, error) {
	if limit <= 0 {
		limit = 20
	}
	if limit > 100 {
		limit = 100
	}
	if page < 1 {
		page = 1
	}

	base := r.db.Builder.
		Select(permissionColumns).
		From("permissions")

	countQuery := r.db.Builder.
		Select("COUNT(*)").
		From("permissions")

	if resource != "" {
		filter := sq.Eq{"resource": resource}
		base = base.Where(filter)
		countQuery = countQuery.Where(filter)
	}

	countSQL, countArgs, err := countQuery.ToSql()
	if err != nil {
		return nil, 0, fmt.Errorf("build count query: %w", err)
	}

	var total int64
	if err := r.db.Pool.QueryRow(ctx, countSQL, countArgs...).Scan(&total); err != nil {
		return nil, 0, fmt.Errorf("count permissions: %w", err)
	}

	offset := (page - 1) * limit
	query, args, err := base.
		OrderBy("resource ASC, action ASC").
		Limit(uint64(limit)).   // #nosec G115 - limit validated
		Offset(uint64(offset)). // #nosec G115 - offset validated
		ToSql()
	if err != nil {
		return nil, 0, fmt.Errorf("build query: %w", err)
	}

	rows, err := r.db.Pool.Query(ctx, query, args...)
	if err != nil {
		return nil, 0, fmt.Errorf("query permissions: %w", err)
	}
	defer rows.Close()

	var perms []accesscontrol.Permission
	for rows.Next() {
		var p accesscontrol.Permission
		if err := rows.Scan(&p.ID, &p.Name, &p.Resource, &p.Action, &p.CreatedAt, &p.UpdatedAt); err != nil {
			return nil, 0, fmt.Errorf("scan permission: %w", err)
		}
		perms = append(perms, p)
	}
	return perms, total, rows.Err()
}
