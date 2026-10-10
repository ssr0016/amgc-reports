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

type RoleRepo struct{ db *database.DB }

func NewRoleRepo(db *database.DB) *RoleRepo { return &RoleRepo{db: db} }

const roleColumns = "id, name, description, created_at, updated_at"

func scanRole(row pgx.Row) (*model.Role, error) {
	var role model.Role
	err := row.Scan(&role.ID, &role.Name, &role.Description, &role.CreatedAt, &role.UpdatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("scan role: %w", err)
	}
	return &role, nil
}

func (r *RoleRepo) findByColumn(ctx context.Context, column string, value interface{}) (*model.Role, error) {
	query, args, err := r.db.Builder.
		Select(roleColumns).
		From("roles").
		Where(sq.Eq{column: value}).
		ToSql()
	if err != nil {
		return nil, fmt.Errorf("build query: %w", err)
	}
	return scanRole(r.db.Pool.QueryRow(ctx, query, args...))
}

func (r *RoleRepo) Create(ctx context.Context, req model.CreateRoleRequest) (*model.Role, error) {
	query, args, err := r.db.Builder.
		Insert("roles").
		Columns("name", "description").
		Values(req.Name, req.Description).
		Suffix("RETURNING " + roleColumns).
		ToSql()
	if err != nil {
		return nil, fmt.Errorf("build query: %w", err)
	}
	return scanRole(r.db.Pool.QueryRow(ctx, query, args...))
}

func (r *RoleRepo) GetByID(ctx context.Context, id int64) (*model.Role, error) {
	return r.findByColumn(ctx, "id", id)
}

func (r *RoleRepo) GetByName(ctx context.Context, name string) (*model.Role, error) {
	return r.findByColumn(ctx, "name", name)
}

func (r *RoleRepo) List(ctx context.Context) ([]model.Role, error) {
	query, args, err := r.db.Builder.
		Select(roleColumns).
		From("roles").
		OrderBy("id ASC").
		ToSql()
	if err != nil {
		return nil, fmt.Errorf("build query: %w", err)
	}

	rows, err := r.db.Pool.Query(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("query roles: %w", err)
	}
	defer rows.Close()

	var roles []model.Role
	for rows.Next() {
		var role model.Role
		if err := rows.Scan(&role.ID, &role.Name, &role.Description, &role.CreatedAt, &role.UpdatedAt); err != nil {
			return nil, fmt.Errorf("scan role: %w", err)
		}
		roles = append(roles, role)
	}
	return roles, rows.Err()
}

func (r *RoleRepo) Update(ctx context.Context, id int64, req model.UpdateRoleRequest) (*model.Role, error) {
	builder := r.db.Builder.Update("roles").Where(sq.Eq{"id": id})

	if req.Name != "" {
		builder = builder.Set("name", req.Name)
	}
	if req.Description != "" {
		builder = builder.Set("description", req.Description)
	}
	builder = builder.Set("updated_at", sq.Expr("NOW()"))

	query, args, err := builder.
		Suffix("RETURNING " + roleColumns).
		ToSql()
	if err != nil {
		return nil, fmt.Errorf("build query: %w", err)
	}
	return scanRole(r.db.Pool.QueryRow(ctx, query, args...))
}

func (r *RoleRepo) Delete(ctx context.Context, id int64) error {
	return deleteByID(ctx, r.db, "roles", id)
}

func (r *RoleRepo) GetPermissions(ctx context.Context, roleID int64) ([]model.Permission, error) {
	query, args, err := r.db.Builder.
		Select("p.id", "p.name", "p.resource", "p.action", "p.created_at", "p.updated_at").
		From("permissions p").
		Join("role_permissions rp ON rp.permission_id = p.id").
		Where(sq.Eq{"rp.role_id": roleID}).
		OrderBy("p.name ASC").
		ToSql()
	if err != nil {
		return nil, fmt.Errorf("build query: %w", err)
	}

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

func (r *RoleRepo) AssignPermission(ctx context.Context, roleID, permissionID int64) error {
	query, args, err := r.db.Builder.
		Insert("role_permissions").
		Columns("role_id", "permission_id").
		Values(roleID, permissionID).
		Suffix("ON CONFLICT (role_id, permission_id) DO NOTHING").
		ToSql()
	if err != nil {
		return fmt.Errorf("build query: %w", err)
	}
	_, err = r.db.Pool.Exec(ctx, query, args...)
	if err != nil {
		return fmt.Errorf("assign permission: %w", err)
	}
	return nil
}

func (r *RoleRepo) RevokePermission(ctx context.Context, roleID, permissionID int64) error {
	query, args, err := r.db.Builder.
		Delete("role_permissions").
		Where(sq.Eq{"role_id": roleID, "permission_id": permissionID}).
		ToSql()
	if err != nil {
		return fmt.Errorf("build query: %w", err)
	}
	_, err = r.db.Pool.Exec(ctx, query, args...)
	if err != nil {
		return fmt.Errorf("revoke permission: %w", err)
	}
	return nil
}

// ListWithPagination returns roles with pagination and total count.
func (r *RoleRepo) ListWithPagination(ctx context.Context, page, limit int) ([]model.Role, int64, error) {
	if limit <= 0 {
		limit = 20
	}
	if limit > 100 {
		limit = 100
	}
	if page < 1 {
		page = 1
	}

	// Get total count
	countQuery, countArgs, err := r.db.Builder.
		Select("COUNT(*)").
		From("roles").
		ToSql()
	if err != nil {
		return nil, 0, fmt.Errorf("build count query: %w", err)
	}

	var total int64
	if err := r.db.Pool.QueryRow(ctx, countQuery, countArgs...).Scan(&total); err != nil {
		return nil, 0, fmt.Errorf("count roles: %w", err)
	}

	// Get paginated data
	offset := (page - 1) * limit
	query, args, err := r.db.Builder.
		Select(roleColumns).
		From("roles").
		OrderBy("id ASC").
		Limit(uint64(limit)).   // #nosec G115 - limit validated
		Offset(uint64(offset)). // #nosec G115 - offset validated
		ToSql()
	if err != nil {
		return nil, 0, fmt.Errorf("build query: %w", err)
	}

	rows, err := r.db.Pool.Query(ctx, query, args...)
	if err != nil {
		return nil, 0, fmt.Errorf("query roles: %w", err)
	}
	defer rows.Close()

	var roles []model.Role
	for rows.Next() {
		var role model.Role
		if err := rows.Scan(&role.ID, &role.Name, &role.Description, &role.CreatedAt, &role.UpdatedAt); err != nil {
			return nil, 0, fmt.Errorf("scan role: %w", err)
		}
		roles = append(roles, role)
	}
	return roles, total, rows.Err()
}
