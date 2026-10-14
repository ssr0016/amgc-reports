package repository

import (
	"context"
	"errors"
	"fmt"
	"time"

	sq "github.com/Masterminds/squirrel"
	"github.com/jackc/pgx/v5"

	"github.com/ssr0016/pastoral-reports/backend/internal/database"
	"github.com/ssr0016/pastoral-reports/backend/internal/model"
)

type UserRepo struct{ db *database.DB }

func NewUserRepo(db *database.DB) *UserRepo { return &UserRepo{db: db} }

const userColumns = "id, email, name, password_hash, role_id, email_verified, email_verified_at, failed_login_attempts, locked_until, created_at, updated_at, worker, area_assignment, church_name"

// scanUser scans a single user from a row.
func scanUser(row pgx.Row) (*model.User, error) {
	var u model.User
	err := row.Scan(&u.ID, &u.Email, &u.Name, &u.PasswordHash, &u.RoleID, &u.EmailVerified, &u.EmailVerifiedAt, &u.FailedLoginAttempts, &u.LockedUntil, &u.CreatedAt, &u.UpdatedAt, &u.Worker, &u.AreaAssignment, &u.ChurchName)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if isUniqueViolation(err) {
		return nil, fmt.Errorf("scan user: %w: %w", ErrUniqueViolation, err)
	}
	if err != nil {
		return nil, fmt.Errorf("scan user: %w", err)
	}
	return &u, nil
}

func (r *UserRepo) GetByID(ctx context.Context, id int64) (*model.User, error) {
	query, args, err := r.db.Builder.
		Select(userColumns).
		From("users").
		Where(sq.Eq{"id": id}).
		ToSql()
	if err != nil {
		return nil, fmt.Errorf("build query: %w", err)
	}

	return scanUser(r.db.Pool.QueryRow(ctx, query, args...))
}

func (r *UserRepo) GetByEmail(ctx context.Context, email string) (*model.User, error) {
	query, args, err := r.db.Builder.
		Select(userColumns).
		From("users").
		Where(sq.Eq{"email": email}).
		ToSql()
	if err != nil {
		return nil, fmt.Errorf("build query: %w", err)
	}

	return scanUser(r.db.Pool.QueryRow(ctx, query, args...))
}

func (r *UserRepo) CreateWithPassword(ctx context.Context, email, name, hash string) (*model.User, error) {
	return r.Create(ctx, r.db.Pool, model.NewUser{Email: email, Name: name, PasswordHash: hash})
}

// Create inserts an account on q (a transaction, so an audit row can share it).
// A zero RoleID means the default role.
func (r *UserRepo) Create(ctx context.Context, q Querier, n model.NewUser) (*model.User, error) {
	roleID := n.RoleID
	if roleID == 0 {
		roleQuery, roleArgs, err := r.db.Builder.
			Select("id").
			From("roles").
			Where(sq.Eq{"name": model.DefaultRoleName}).
			ToSql()
		if err != nil {
			return nil, fmt.Errorf("build role query: %w", err)
		}
		if err := q.QueryRow(ctx, roleQuery, roleArgs...).Scan(&roleID); err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				return nil, fmt.Errorf("get default role %q: %w", model.DefaultRoleName, ErrDefaultRoleMissing)
			}
			return nil, fmt.Errorf("get default role: %w", err)
		}
	}

	query, args, err := r.db.Builder.
		Insert("users").
		Columns("email", "name", "password_hash", "role_id", "worker", "area_assignment", "church_name").
		Values(n.Email, n.Name, n.PasswordHash, roleID, nullable(n.Worker), nullable(n.AreaAssignment), nullable(n.ChurchName)).
		Suffix("RETURNING " + userColumns).
		ToSql()
	if err != nil {
		return nil, fmt.Errorf("build query: %w", err)
	}

	u, err := scanUser(q.QueryRow(ctx, query, args...))
	if isForeignKeyViolation(err) {
		return nil, fmt.Errorf("create user: %w: %w", ErrForeignKeyViolation, err)
	}
	return u, err
}

// UpdateProfile replaces a user's name and profile text on q; the password and
// role are not touched. Empty profile text is stored as NULL.
func (r *UserRepo) UpdateProfile(ctx context.Context, q Querier, id int64, p model.UserProfile) (*model.User, error) {
	query, args, err := r.db.Builder.
		Update("users").
		Set("name", p.Name).
		Set("worker", nullable(p.Worker)).
		Set("area_assignment", nullable(p.AreaAssignment)).
		Set("church_name", nullable(p.ChurchName)).
		Set("updated_at", sq.Expr("NOW()")).
		Where(sq.Eq{"id": id}).
		Suffix("RETURNING " + userColumns).
		ToSql()
	if err != nil {
		return nil, fmt.Errorf("build query: %w", err)
	}
	u, err := scanUser(q.QueryRow(ctx, query, args...))
	if err != nil {
		return nil, err
	}
	if u == nil {
		return nil, fmt.Errorf("users: %w", ErrNotFound)
	}
	return u, nil
}

func (r *UserRepo) List(ctx context.Context, emailFilter string, limit int) ([]model.User, error) {
	// Validate limit to prevent overflow and DoS
	if limit <= 0 {
		limit = 20
	}
	if limit > 100 {
		limit = 100
	}

	q := r.db.Builder.
		Select(userColumns).
		From("users").
		OrderBy("id DESC").
		Limit(uint64(limit)) // #nosec G115 - limit is validated to be 1..100

	if emailFilter != "" {
		q = q.Where(sq.ILike{"email": "%" + emailFilter + "%"})
	}

	query, args, err := q.ToSql()
	if err != nil {
		return nil, fmt.Errorf("build query: %w", err)
	}

	rows, err := r.db.Pool.Query(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("query: %w", err)
	}
	defer rows.Close()

	var users []model.User
	for rows.Next() {
		var u model.User
		if err := rows.Scan(&u.ID, &u.Email, &u.Name, &u.PasswordHash, &u.RoleID, &u.EmailVerified, &u.EmailVerifiedAt, &u.FailedLoginAttempts, &u.LockedUntil, &u.CreatedAt, &u.UpdatedAt, &u.Worker, &u.AreaAssignment, &u.ChurchName); err != nil {
			return nil, fmt.Errorf("scan row: %w", err)
		}
		users = append(users, u)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("rows error: %w", err)
	}
	return users, nil
}

// UpdateRole changes a user's role.
func (r *UserRepo) UpdateRole(ctx context.Context, userID, roleID int64) error {
	query, args, err := r.db.Builder.
		Update("users").
		Set("role_id", roleID).
		Set("updated_at", sq.Expr("NOW()")).
		Where(sq.Eq{"id": userID}).
		ToSql()
	if err != nil {
		return fmt.Errorf("build query: %w", err)
	}

	result, err := r.db.Pool.Exec(ctx, query, args...)
	if isForeignKeyViolation(err) {
		return fmt.Errorf("update role: %w: %w", ErrForeignKeyViolation, err)
	}
	if err != nil {
		return fmt.Errorf("update role: %w", err)
	}
	if result.RowsAffected() == 0 {
		return fmt.Errorf("users: %w", ErrNotFound)
	}
	return nil
}

// GetWithRole returns a user with their role loaded.
func (r *UserRepo) GetWithRole(ctx context.Context, id int64) (*model.User, error) {
	query, args, err := r.db.Builder.
		Select(
			"u.id", "u.email", "u.name", "u.password_hash", "u.role_id",
			"u.created_at", "u.updated_at", "u.worker", "u.area_assignment", "u.church_name",
			"r.id", "r.name", "r.description", "r.created_at", "r.updated_at",
		).
		From("users u").
		LeftJoin("roles r ON r.id = u.role_id").
		Where(sq.Eq{"u.id": id}).
		ToSql()
	if err != nil {
		return nil, fmt.Errorf("build query: %w", err)
	}

	var u model.User
	var rID *int64
	var rName, rDesc *string
	var rCreatedAt, rUpdatedAt *time.Time

	err = r.db.Pool.QueryRow(ctx, query, args...).Scan(
		&u.ID, &u.Email, &u.Name, &u.PasswordHash, &u.RoleID,
		&u.CreatedAt, &u.UpdatedAt, &u.Worker, &u.AreaAssignment, &u.ChurchName,
		&rID, &rName, &rDesc, &rCreatedAt, &rUpdatedAt,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("query user: %w", err)
	}

	// Attach role if loaded
	if rID != nil {
		u.Role = &model.Role{
			ID:          *rID,
			Name:        *rName,
			Description: *rDesc,
			CreatedAt:   *rCreatedAt,
			UpdatedAt:   *rUpdatedAt,
		}
	}

	return &u, nil
}

// ListWithPagination returns users with pagination and total count.
func (r *UserRepo) ListWithPagination(ctx context.Context, emailFilter string, page, limit int) ([]model.User, int64, error) {
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
		Select(userColumns).
		From("users")

	countQuery := r.db.Builder.
		Select("COUNT(*)").
		From("users")

	if emailFilter != "" {
		filter := sq.ILike{"email": "%" + emailFilter + "%"}
		base = base.Where(filter)
		countQuery = countQuery.Where(filter)
	}

	countSQL, countArgs, err := countQuery.ToSql()
	if err != nil {
		return nil, 0, fmt.Errorf("build count query: %w", err)
	}

	var total int64
	if err := r.db.Pool.QueryRow(ctx, countSQL, countArgs...).Scan(&total); err != nil {
		return nil, 0, fmt.Errorf("count users: %w", err)
	}

	offset := (page - 1) * limit
	query, args, err := base.
		OrderBy("id DESC").
		Limit(uint64(limit)).   // #nosec G115 - limit validated
		Offset(uint64(offset)). // #nosec G115 - offset validated
		ToSql()
	if err != nil {
		return nil, 0, fmt.Errorf("build query: %w", err)
	}

	rows, err := r.db.Pool.Query(ctx, query, args...)
	if err != nil {
		return nil, 0, fmt.Errorf("query: %w", err)
	}
	defer rows.Close()

	var users []model.User
	for rows.Next() {
		var u model.User
		if err := rows.Scan(&u.ID, &u.Email, &u.Name, &u.PasswordHash, &u.RoleID, &u.EmailVerified, &u.EmailVerifiedAt, &u.FailedLoginAttempts, &u.LockedUntil, &u.CreatedAt, &u.UpdatedAt, &u.Worker, &u.AreaAssignment, &u.ChurchName); err != nil {
			return nil, 0, fmt.Errorf("scan user: %w", err)
		}
		users = append(users, u)
	}
	return users, total, rows.Err()
}

// MarkEmailVerified marks a user's email as verified.
func (r *UserRepo) MarkEmailVerified(ctx context.Context, userID int64) error {
	query, args, err := r.db.Builder.
		Update("users").
		Set("email_verified", true).
		Set("email_verified_at", sq.Expr("NOW()")).
		Set("updated_at", sq.Expr("NOW()")).
		Where(sq.Eq{"id": userID}).
		ToSql()
	if err != nil {
		return fmt.Errorf("build query: %w", err)
	}

	result, err := r.db.Pool.Exec(ctx, query, args...)
	if err != nil {
		return fmt.Errorf("mark email verified: %w", err)
	}
	if result.RowsAffected() == 0 {
		return fmt.Errorf("users: %w", ErrNotFound)
	}
	return nil
}

// UpdatePassword updates a user's password hash and revokes all of the user's
// sessions, in one transaction: a changed password never leaves the old
// sessions alive, and a failed revocation leaves the old password in place.
func (r *UserRepo) UpdatePassword(ctx context.Context, userID int64, hash string) error {
	query, args, err := r.db.Builder.
		Update("users").
		Set("password_hash", hash).
		Set("updated_at", sq.Expr("NOW()")).
		Where(sq.Eq{"id": userID}).
		ToSql()
	if err != nil {
		return fmt.Errorf("build query: %w", err)
	}

	tx, err := r.db.Pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }() // no-op once committed

	result, err := tx.Exec(ctx, query, args...)
	if err != nil {
		return fmt.Errorf("update password: %w", err)
	}
	if result.RowsAffected() == 0 {
		return fmt.Errorf("users: %w", ErrNotFound)
	}

	// A tombstone, not a delete: see the session store.
	if _, err := tx.Exec(ctx,
		`UPDATE sessions SET revoked_at = NOW() WHERE user_id = $1 AND revoked_at IS NULL`, userID); err != nil {
		return fmt.Errorf("revoke sessions: %w", err)
	}

	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("commit: %w", err)
	}
	return nil
}

// RecordFailedLogin increments failed_login_attempts and locks account if threshold reached.
func (r *UserRepo) RecordFailedLogin(ctx context.Context, userID int64, maxAttempts int, lockDuration time.Duration) error {
	query, args, err := r.db.Builder.
		Update("users").
		Set("failed_login_attempts", sq.Expr("failed_login_attempts + 1")).
		Set("locked_until", sq.Expr("CASE WHEN failed_login_attempts + 1 >= ? THEN NOW() + ?::interval ELSE locked_until END", maxAttempts, lockDuration.String())).
		Set("updated_at", sq.Expr("NOW()")).
		Where(sq.Eq{"id": userID}).
		ToSql()
	if err != nil {
		return fmt.Errorf("build query: %w", err)
	}
	result, err := r.db.Pool.Exec(ctx, query, args...)
	if err != nil {
		return fmt.Errorf("record failed login: %w", err)
	}
	if result.RowsAffected() == 0 {
		return fmt.Errorf("users: %w", ErrNotFound)
	}
	return nil
}

// ResetLoginAttempts clears failed attempts and unlocks account.
func (r *UserRepo) ResetLoginAttempts(ctx context.Context, userID int64) error {
	query, args, err := r.db.Builder.
		Update("users").
		Set("failed_login_attempts", 0).
		Set("locked_until", nil).
		Set("updated_at", sq.Expr("NOW()")).
		Where(sq.Eq{"id": userID}).
		ToSql()
	if err != nil {
		return fmt.Errorf("build query: %w", err)
	}
	result, err := r.db.Pool.Exec(ctx, query, args...)
	if err != nil {
		return fmt.Errorf("reset login attempts: %w", err)
	}
	if result.RowsAffected() == 0 {
		return fmt.Errorf("users: %w", ErrNotFound)
	}
	return nil
}
