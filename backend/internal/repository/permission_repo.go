package repository

import (
	"context"
	"errors"
	"fmt"

	sq "github.com/Masterminds/squirrel"
	"github.com/jackc/pgx/v5"

	"github.com/ssr0016/template/internal/database"
	"github.com/ssr0016/template/internal/model"
)

type PermissionRepo struct{ db *database.DB }

func NewPermissionRepo(db *database.DB) *PermissionRepo { return &PermissionRepo{db: db} }

const permissionColumns = "id, name, resource, action, created_at, updated_at"

// scanPermission scans a single permission from a row.
func scanPermission(row pgx.Row) (*model.Permission, error) {
	var p model.Permission
	err := row.Scan(&p.ID, &p.Name, &p.Resource, &p.Action, &p.CreatedAt, &p.UpdatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("scan permission: %w", err)
	}
	return &p, nil
}

// findByColumn is a helper for fetching a single permission by any column.
func (r *PermissionRepo) findByColumn(ctx context.Context, column string, value interface{}) (*model.Permission, error) {
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

func (r *PermissionRepo) Create(ctx context.Context, name, resource, action string) (*model.Permission, error) {
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

func (r *PermissionRepo) GetByID(ctx context.Context, id int64) (*model.Permission, error) {
	return r.findByColumn(ctx, "id", id)
}

func (r *PermissionRepo) GetByName(ctx context.Context, name string) (*model.Permission, error) {
	return r.findByColumn(ctx, "name", name)
}

func (r *PermissionRepo) List(ctx context.Context) ([]model.Permission, error) {
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

func (r *PermissionRepo) ListByResource(ctx context.Context, resource string) ([]model.Permission, error) {
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

func (r *PermissionRepo) Delete(ctx context.Context, id int64) error {
	return deleteByID(ctx, r.db, "permissions", id)
}

func (r *PermissionRepo) scanPermissions(ctx context.Context, query string, args ...interface{}) ([]model.Permission, error) {
	rows, err := r.db.Pool.Query(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("query permissions: %w", err)
	}
	defer rows.Close()

	var perms []model.Permission
	for rows.Next() {
		var p model.Permission
		if err := rows.Scan(&p.ID, &p.Name, &p.Resource, &p.Action, &p.CreatedAt, &p.UpdatedAt); err != nil {
			return nil, fmt.Errorf("scan permission: %w", err)
		}
		perms = append(perms, p)
	}
	return perms, rows.Err()
}

// ListWithPagination returns permissions with pagination and total count.
func (r *PermissionRepo) ListWithPagination(ctx context.Context, resource string, page, limit int) ([]model.Permission, int64, error) {
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

	var perms []model.Permission
	for rows.Next() {
		var p model.Permission
		if err := rows.Scan(&p.ID, &p.Name, &p.Resource, &p.Action, &p.CreatedAt, &p.UpdatedAt); err != nil {
			return nil, 0, fmt.Errorf("scan permission: %w", err)
		}
		perms = append(perms, p)
	}
	return perms, total, rows.Err()
}
