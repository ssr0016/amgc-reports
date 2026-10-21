package userimpl

import (
	"context"
	"errors"
	"fmt"
	"time"

	sq "github.com/Masterminds/squirrel"
	"github.com/jackc/pgx/v5"

	"github.com/ssr0016/pastoral-reports/backend/internal/accesscontrol"
	"github.com/ssr0016/pastoral-reports/backend/internal/database"
	"github.com/ssr0016/pastoral-reports/backend/internal/user"
)

type store struct{ db *database.DB }

// NewStore returns the user.Store over db.
func NewStore(db *database.DB) user.Store { return &store{db: db} }

const userColumns = "id, email, name, password_hash, role_id, email_verified, email_verified_at, failed_login_attempts, locked_until, created_at, updated_at, area_assignment, church_name, status"

// scanUser scans a single user from a row.
func scanUser(row pgx.Row) (*user.User, error) {
	var u user.User
	err := row.Scan(&u.ID, &u.Email, &u.Name, &u.PasswordHash, &u.RoleID, &u.EmailVerified, &u.EmailVerifiedAt, &u.FailedLoginAttempts, &u.LockedUntil, &u.CreatedAt, &u.UpdatedAt, &u.AreaAssignment, &u.ChurchName, &u.Status)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if database.IsUniqueViolation(err) {
		return nil, fmt.Errorf("scan user: %w: %w", database.ErrUniqueViolation, err)
	}
	if err != nil {
		return nil, fmt.Errorf("scan user: %w", err)
	}
	return &u, nil
}

func (r *store) GetByID(ctx context.Context, id int64) (*user.User, error) {
	query, args, err := r.db.Builder.
		Select(userColumns).
		From("users").
		Where(sq.Eq{"id": id}).
		ToSql()
	if err != nil {
		return nil, fmt.Errorf("build query: %w", err)
	}

	return scanUser(r.db.Querier(ctx).QueryRow(ctx, query, args...))
}

func (r *store) GetByEmail(ctx context.Context, email string) (*user.User, error) {
	query, args, err := r.db.Builder.
		Select(userColumns).
		From("users").
		Where(sq.Eq{"email": email}).
		ToSql()
	if err != nil {
		return nil, fmt.Errorf("build query: %w", err)
	}

	return scanUser(r.db.Querier(ctx).QueryRow(ctx, query, args...))
}

func (r *store) CreateWithPassword(ctx context.Context, email, name, hash string) (*user.User, error) {
	return r.Create(ctx, user.NewUser{Email: email, Name: name, PasswordHash: hash})
}

// Create inserts an account. In a transaction ctx it joins it, so an audit row can share it.
// A zero RoleID means the default role.
func (r *store) Create(ctx context.Context, n user.NewUser) (*user.User, error) {
	roleID := n.RoleID
	if roleID == 0 {
		roleQuery, roleArgs, err := r.db.Builder.
			Select("id").
			From("roles").
			Where(sq.Eq{"name": accesscontrol.DefaultRoleName}).
			ToSql()
		if err != nil {
			return nil, fmt.Errorf("build role query: %w", err)
		}
		if err := r.db.Querier(ctx).QueryRow(ctx, roleQuery, roleArgs...).Scan(&roleID); err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				return nil, fmt.Errorf("get default role %q: %w", accesscontrol.DefaultRoleName, accesscontrol.ErrDefaultRoleMissing)
			}
			return nil, fmt.Errorf("get default role: %w", err)
		}
	}

	query, args, err := r.db.Builder.
		Insert("users").
		Columns("email", "name", "password_hash", "role_id", "area_assignment", "church_name").
		Values(n.Email, n.Name, n.PasswordHash, roleID, nullable(n.AreaAssignment), nullable(n.ChurchName)).
		Suffix("RETURNING " + userColumns).
		ToSql()
	if err != nil {
		return nil, fmt.Errorf("build query: %w", err)
	}

	u, err := scanUser(r.db.Querier(ctx).QueryRow(ctx, query, args...))
	if database.IsForeignKeyViolation(err) {
		return nil, fmt.Errorf("create user: %w: %w", database.ErrForeignKeyViolation, err)
	}
	return u, err
}

// UpdateProfile replaces a user's name and profile text; the password and
// role are not touched. Empty profile text is stored as NULL.
func (r *store) UpdateProfile(ctx context.Context, id int64, p user.UserProfile) (*user.User, error) {
	query, args, err := r.db.Builder.
		Update("users").
		Set("name", p.Name).
		Set("area_assignment", nullable(p.AreaAssignment)).
		Set("church_name", nullable(p.ChurchName)).
		Set("updated_at", sq.Expr("NOW()")).
		Where(sq.Eq{"id": id}).
		Suffix("RETURNING " + userColumns).
		ToSql()
	if err != nil {
		return nil, fmt.Errorf("build query: %w", err)
	}
	u, err := scanUser(r.db.Querier(ctx).QueryRow(ctx, query, args...))
	if err != nil {
		return nil, err
	}
	if u == nil {
		return nil, fmt.Errorf("users: %w", database.ErrNotFound)
	}
	return u, nil
}

func (r *store) List(ctx context.Context, emailFilter, status string, limit int) ([]user.User, error) {
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
	if status != "" {
		q = q.Where(sq.Eq{"status": status})
	}

	query, args, err := q.ToSql()
	if err != nil {
		return nil, fmt.Errorf("build query: %w", err)
	}

	rows, err := r.db.Querier(ctx).Query(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("query: %w", err)
	}
	defer rows.Close()

	var users []user.User
	for rows.Next() {
		var u user.User
		if err := rows.Scan(&u.ID, &u.Email, &u.Name, &u.PasswordHash, &u.RoleID, &u.EmailVerified, &u.EmailVerifiedAt, &u.FailedLoginAttempts, &u.LockedUntil, &u.CreatedAt, &u.UpdatedAt, &u.AreaAssignment, &u.ChurchName, &u.Status); err != nil {
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
func (r *store) UpdateRole(ctx context.Context, userID, roleID int64) error {
	query, args, err := r.db.Builder.
		Update("users").
		Set("role_id", roleID).
		Set("updated_at", sq.Expr("NOW()")).
		Where(sq.Eq{"id": userID}).
		ToSql()
	if err != nil {
		return fmt.Errorf("build query: %w", err)
	}

	result, err := r.db.Querier(ctx).Exec(ctx, query, args...)
	if database.IsForeignKeyViolation(err) {
		return fmt.Errorf("update role: %w: %w", database.ErrForeignKeyViolation, err)
	}
	if err != nil {
		return fmt.Errorf("update role: %w", err)
	}
	if result.RowsAffected() == 0 {
		return fmt.Errorf("users: %w", database.ErrNotFound)
	}
	return nil
}

// GetWithRole returns a user with their role loaded.
func (r *store) GetWithRole(ctx context.Context, id int64) (*user.User, error) {
	query, args, err := r.db.Builder.
		Select(
			"u.id", "u.email", "u.name", "u.password_hash", "u.role_id",
			"u.created_at", "u.updated_at", "u.area_assignment", "u.church_name", "u.status", "u.tokens_valid_after",
			"r.id", "r.name", "r.description", "r.created_at", "r.updated_at",
		).
		From("users u").
		LeftJoin("roles r ON r.id = u.role_id").
		Where(sq.Eq{"u.id": id}).
		ToSql()
	if err != nil {
		return nil, fmt.Errorf("build query: %w", err)
	}

	var u user.User
	var rID *int64
	var rName, rDesc *string
	var rCreatedAt, rUpdatedAt *time.Time

	err = r.db.Querier(ctx).QueryRow(ctx, query, args...).Scan(
		&u.ID, &u.Email, &u.Name, &u.PasswordHash, &u.RoleID,
		&u.CreatedAt, &u.UpdatedAt, &u.AreaAssignment, &u.ChurchName, &u.Status, &u.TokensValidAfter,
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
		u.Role = &accesscontrol.Role{
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
func (r *store) ListWithPagination(ctx context.Context, emailFilter string, page, limit int) ([]user.User, int64, error) {
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
	if err := r.db.Querier(ctx).QueryRow(ctx, countSQL, countArgs...).Scan(&total); err != nil {
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

	rows, err := r.db.Querier(ctx).Query(ctx, query, args...)
	if err != nil {
		return nil, 0, fmt.Errorf("query: %w", err)
	}
	defer rows.Close()

	var users []user.User
	for rows.Next() {
		var u user.User
		if err := rows.Scan(&u.ID, &u.Email, &u.Name, &u.PasswordHash, &u.RoleID, &u.EmailVerified, &u.EmailVerifiedAt, &u.FailedLoginAttempts, &u.LockedUntil, &u.CreatedAt, &u.UpdatedAt, &u.AreaAssignment, &u.ChurchName, &u.Status); err != nil {
			return nil, 0, fmt.Errorf("scan user: %w", err)
		}
		users = append(users, u)
	}
	return users, total, rows.Err()
}

// MarkEmailVerified marks a user's email as verified.
func (r *store) MarkEmailVerified(ctx context.Context, userID int64) error {
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

	result, err := r.db.Querier(ctx).Exec(ctx, query, args...)
	if err != nil {
		return fmt.Errorf("mark email verified: %w", err)
	}
	if result.RowsAffected() == 0 {
		return fmt.Errorf("users: %w", database.ErrNotFound)
	}
	return nil
}

// UpdatePassword updates a user's password hash and ends all of the user's
// sign-ins (refresh tokens, and access tokens through tokens_valid_after), in one
// transaction: a changed password never leaves the old sign-ins alive, and a
// failed revocation leaves the old password in place.
func (r *store) UpdatePassword(ctx context.Context, userID int64, hash string) error {
	now := revocationTime()
	query, args, err := r.db.Builder.
		Update("users").
		Set("password_hash", hash).
		Set("updated_at", sq.Expr("NOW()")).
		Set("tokens_valid_after", now).
		Where(sq.Eq{"id": userID}).
		ToSql()
	if err != nil {
		return fmt.Errorf("build query: %w", err)
	}

	return r.db.InTransaction(ctx, func(ctx context.Context) error {
		q := r.db.Querier(ctx)
		result, err := q.Exec(ctx, query, args...)
		if err != nil {
			return fmt.Errorf("update password: %w", err)
		}
		if result.RowsAffected() == 0 {
			return fmt.Errorf("users: %w", database.ErrNotFound)
		}

		if _, err := q.Exec(ctx,
			`UPDATE refresh_tokens SET revoked_at = $2 WHERE user_id = $1 AND revoked_at IS NULL`, userID, now); err != nil {
			return fmt.Errorf("revoke sessions: %w", err)
		}
		return nil
	})
}

// UpdatePasswordKeepSession sets the password and ends the user's other sign-ins:
// every refresh-token family except keepFamily (the one the user is changing it
// from) is revoked, and tokens_valid_after is moved up so their access tokens stop
// working at once. The caller issues a fresh access token for keepFamily. It joins
// the transaction in ctx, so the caller can put it in one transaction with the
// audit row, and it returns how many sign-ins it ended. A missing user is
// database.ErrNotFound and revokes nothing. UpdatePassword, by contrast, ends every
// sign-in (used when the user is not signed in).
func (r *store) UpdatePasswordKeepSession(ctx context.Context, userID int64, hash, keepFamily string) (int64, error) {
	now := revocationTime()
	query, args, err := r.db.Builder.
		Update("users").
		Set("password_hash", hash).
		Set("updated_at", sq.Expr("NOW()")).
		Set("tokens_valid_after", now).
		Where(sq.Eq{"id": userID}).
		ToSql()
	if err != nil {
		return 0, fmt.Errorf("build query: %w", err)
	}
	q := r.db.Querier(ctx)
	result, err := q.Exec(ctx, query, args...)
	if err != nil {
		return 0, fmt.Errorf("update password: %w", err)
	}
	if result.RowsAffected() == 0 {
		return 0, fmt.Errorf("users: %w", database.ErrNotFound)
	}

	revoked, err := q.Exec(ctx,
		`UPDATE refresh_tokens SET revoked_at = $3 WHERE user_id = $1 AND family_id <> $2 AND revoked_at IS NULL`,
		userID, keepFamily, now)
	if err != nil {
		return 0, fmt.Errorf("revoke sessions: %w", err)
	}
	return revoked.RowsAffected(), nil
}

// revocationTime is the moment a revocation takes effect. It is cut to the
// millisecond, the precision of an access token's iat, so a token signed right
// after the revocation is never older than it. It comes from the app clock, the
// same one that signs tokens, not the database's.
func revocationTime() time.Time { return time.Now().Truncate(time.Millisecond) }

// RecordFailedLogin increments failed_login_attempts and locks account if threshold reached.
func (r *store) RecordFailedLogin(ctx context.Context, userID int64, maxAttempts int, lockDuration time.Duration) error {
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
	result, err := r.db.Querier(ctx).Exec(ctx, query, args...)
	if err != nil {
		return fmt.Errorf("record failed login: %w", err)
	}
	if result.RowsAffected() == 0 {
		return fmt.Errorf("users: %w", database.ErrNotFound)
	}
	return nil
}

// ResetLoginAttempts clears failed attempts and unlocks account.
func (r *store) ResetLoginAttempts(ctx context.Context, userID int64) error {
	return r.ClearLockout(ctx, userID)
}

// ClearLockout is ResetLoginAttempts: failed_login_attempts to 0 and locked_until to
// NULL. A missing user is database.ErrNotFound.
func (r *store) ClearLockout(ctx context.Context, userID int64) error {
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
	result, err := r.db.Querier(ctx).Exec(ctx, query, args...)
	if err != nil {
		return fmt.Errorf("reset login attempts: %w", err)
	}
	if result.RowsAffected() == 0 {
		return fmt.Errorf("users: %w", database.ErrNotFound)
	}
	return nil
}

// UpdateStatus sets a user's status (in a transaction ctx, so the audit row and the
// session revoke share it) and returns the updated user. A missing user is database.ErrNotFound.
func (r *store) UpdateStatus(ctx context.Context, id int64, status string) (*user.User, error) {
	query, args, err := r.db.Builder.
		Update("users").
		Set("status", status).
		Set("updated_at", sq.Expr("NOW()")).
		Where(sq.Eq{"id": id}).
		Suffix("RETURNING " + userColumns).
		ToSql()
	if err != nil {
		return nil, fmt.Errorf("build query: %w", err)
	}
	u, err := scanUser(r.db.Querier(ctx).QueryRow(ctx, query, args...))
	if err != nil {
		return nil, err
	}
	if u == nil {
		return nil, fmt.Errorf("users: %w", database.ErrNotFound)
	}
	return u, nil
}

// RevokeSessions ends all of a user's sign-ins (refresh tokens and, through
// tokens_valid_after, access tokens) and returns how many refresh tokens it revoked.
func (r *store) RevokeSessions(ctx context.Context, userID int64) (int64, error) {
	now := revocationTime()
	var ended int64
	// The users row goes first, always: Refresh holds a share lock on it while it
	// rotates (see LockUser), so a revocation waits for a rotation in flight and
	// then sees the token it created. Its own transaction when the caller has none,
	// so the two statements cannot be separated by a refresh.
	err := r.db.InTransaction(ctx, func(ctx context.Context) error {
		q := r.db.Querier(ctx)
		if _, err := q.Exec(ctx, `UPDATE users SET tokens_valid_after = $2 WHERE id = $1`, userID, now); err != nil {
			return fmt.Errorf("revoke access tokens: %w", err)
		}
		res, err := q.Exec(ctx,
			`UPDATE refresh_tokens SET revoked_at = $2 WHERE user_id = $1 AND revoked_at IS NULL`, userID, now)
		if err != nil {
			return fmt.Errorf("revoke sessions: %w", err)
		}
		ended = res.RowsAffected()
		return nil
	})
	return ended, err
}

// LockUser takes a share lock on the user's row until the transaction in ctx ends.
// Refresh calls it before it locks the token, so it runs one at a time with every
// revocation (UpdatePassword, UpdatePasswordKeepSession, RevokeSessions), which
// update the users row first. One lock order, users then refresh_tokens, so no
// deadlock. Two refreshes share the lock and meet on the token row. A missing
// user is database.ErrNotFound.
func (r *store) LockUser(ctx context.Context, userID int64) error {
	var one int
	err := r.db.Querier(ctx).QueryRow(ctx, `SELECT 1 FROM users WHERE id = $1 FOR SHARE`, userID).Scan(&one)
	if errors.Is(err, pgx.ErrNoRows) {
		return fmt.Errorf("users: %w", database.ErrNotFound)
	}
	if err != nil {
		return fmt.Errorf("lock user: %w", err)
	}
	return nil
}

// LockActiveAdmins returns the ids of the active admins, in id order, and locks
// their rows until the transaction in ctx ends. Taking the locks in a fixed order
// means two admins changing each other's status at once cannot both pass the
// last-admin check.
func (r *store) LockActiveAdmins(ctx context.Context) ([]int64, error) {
	rows, err := r.db.Querier(ctx).Query(ctx,
		`SELECT u.id FROM users u JOIN roles r ON r.id = u.role_id
		 WHERE r.name = 'admin' AND u.status = 'active' ORDER BY u.id FOR UPDATE OF u`)
	if err != nil {
		return nil, fmt.Errorf("lock active admins: %w", err)
	}
	defer rows.Close()
	var ids []int64
	for rows.Next() {
		var id int64
		if err := rows.Scan(&id); err != nil {
			return nil, fmt.Errorf("scan admin id: %w", err)
		}
		ids = append(ids, id)
	}
	return ids, rows.Err()
}

// nullable stores an empty string as NULL (D-8).
func nullable(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}
