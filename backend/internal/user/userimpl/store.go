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
	"github.com/ssr0016/pastoral-reports/backend/pkg/pagination"
)

type store struct{ db *database.DB }

// NewStore returns the user.Store over db.
func NewStore(db *database.DB) user.Store { return &store{db: db} }

const userColumns = "id, email, name, password_hash, role_id, email_verified, email_verified_at, created_at, updated_at, area_assignment, church_name, status, deleted_at, deleted_by, deleted_reason, avatar_url"

// scanUser scans a single user from a row.
func scanUser(row pgx.Row) (*user.User, error) {
	var u user.User
	err := row.Scan(&u.ID, &u.Email, &u.Name, &u.PasswordHash, &u.RoleID, &u.EmailVerified, &u.EmailVerifiedAt, &u.CreatedAt, &u.UpdatedAt, &u.AreaAssignment, &u.ChurchName, &u.Status, &u.DeletedAt, &u.DeletedBy, &u.DeletedReason, &u.AvatarURL)
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

// UpdateAvatar serializes replacements so each upload cleans up the URL it replaced.
func (r *store) UpdateAvatar(ctx context.Context, id int64, url string) (*user.User, string, error) {
	var previous *string
	var updated *user.User
	err := r.db.InTransaction(ctx, func(ctx context.Context) error {
		err := r.db.Querier(ctx).QueryRow(ctx,
			"SELECT avatar_url FROM users WHERE id = $1 FOR UPDATE", id).Scan(&previous)
		if errors.Is(err, pgx.ErrNoRows) {
			return fmt.Errorf("users: %w", database.ErrNotFound)
		}
		if err != nil {
			return fmt.Errorf("lock avatar: %w", err)
		}
		updated, err = r.setColumn(ctx, id, "avatar_url", url)
		return err
	})
	if err != nil {
		return nil, "", err
	}
	previousURL := ""
	if previous != nil {
		previousURL = *previous
	}
	return updated, previousURL, nil
}

func (r *store) List(ctx context.Context, emailFilter, status string, roleID int64, pg pagination.Params) ([]user.User, int64, error) {
	where := sq.And{}
	if emailFilter != "" {
		where = append(where, sq.ILike{"email": "%" + emailFilter + "%"})
	}
	if roleID != 0 {
		where = append(where, sq.Eq{"role_id": roleID})
	}
	// "deleted" is a list filter, not a status (D-A11): every other value hides deleted users.
	switch {
	case status == "deleted":
		where = append(where, sq.NotEq{"deleted_at": nil})
	case status != "":
		where = append(where, sq.Eq{"status": status, "deleted_at": nil})
	default:
		where = append(where, sq.Eq{"deleted_at": nil})
	}

	countSQL, countArgs, err := r.db.Builder.Select("COUNT(*)").From("users").Where(where).ToSql()
	if err != nil {
		return nil, 0, fmt.Errorf("build count query: %w", err)
	}
	var total int64
	if err := r.db.Querier(ctx).QueryRow(ctx, countSQL, countArgs...).Scan(&total); err != nil {
		return nil, 0, fmt.Errorf("count users: %w", err)
	}

	query, args, err := r.db.Builder.
		Select(userColumns).
		From("users").
		Where(where).
		OrderBy("id DESC").
		Limit(uint64(pg.Limit)).     // #nosec G115 - pagination.FromQuery keeps limit in 1..100
		Offset(uint64(pg.Offset())). // #nosec G115 - page >= 1, so the offset is never negative
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
		if err := rows.Scan(&u.ID, &u.Email, &u.Name, &u.PasswordHash, &u.RoleID, &u.EmailVerified, &u.EmailVerifiedAt, &u.CreatedAt, &u.UpdatedAt, &u.AreaAssignment, &u.ChurchName, &u.Status, &u.DeletedAt, &u.DeletedBy, &u.DeletedReason, &u.AvatarURL); err != nil {
			return nil, 0, fmt.Errorf("scan row: %w", err)
		}
		users = append(users, u)
	}
	if err := rows.Err(); err != nil {
		return nil, 0, fmt.Errorf("rows error: %w", err)
	}
	return users, total, nil
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

// ReassignRole moves every account holding from to to, deleted ones included.
func (r *store) ReassignRole(ctx context.Context, from, to int64) ([]user.User, error) {
	rows, err := r.db.Querier(ctx).Query(ctx,
		`UPDATE users SET role_id = $2, updated_at = NOW() WHERE role_id = $1 RETURNING `+userColumns, from, to)
	if err != nil {
		return nil, fmt.Errorf("reassign role: %w", err)
	}
	defer rows.Close()
	var users []user.User
	for rows.Next() {
		u, err := scanUser(rows)
		if err != nil {
			return nil, err
		}
		users = append(users, *u)
	}
	if database.IsForeignKeyViolation(rows.Err()) {
		return nil, fmt.Errorf("reassign role: %w: %w", database.ErrForeignKeyViolation, rows.Err())
	}
	return users, rows.Err()
}

// GetWithRole returns a user with their role loaded.
func (r *store) GetWithRole(ctx context.Context, id int64) (*user.User, error) {
	return r.withRole(ctx, id, nil)
}

// GetCaller returns the user with their role and its permission names, in one query.
func (r *store) GetCaller(ctx context.Context, id int64) (*user.Caller, error) {
	var names []string
	u, err := r.withRole(ctx, id, &names)
	if err != nil || u == nil {
		return nil, err
	}
	perms := make(map[string]bool, len(names))
	for _, n := range names {
		perms[n] = true
	}
	return &user.Caller{User: u, Permissions: perms}, nil
}

// withRole reads the user and role; a non-nil perms also gets the role's permission names.
func (r *store) withRole(ctx context.Context, id int64, perms *[]string) (*user.User, error) {
	sb := r.db.Builder.
		Select(
			"u.id", "u.email", "u.name", "u.password_hash", "u.role_id",
			"u.created_at", "u.updated_at", "u.area_assignment", "u.church_name", "u.status", "u.tokens_valid_after", "u.deleted_at", "u.avatar_url",
			"r.id", "r.name", "r.description", "r.created_at", "r.updated_at",
		).
		From("users u").
		LeftJoin("roles r ON r.id = u.role_id").
		Where(sq.Eq{"u.id": id})
	if perms != nil {
		// ARRAY(...) is never NULL: a user without a role or permissions gets {}.
		sb = sb.Column("ARRAY(SELECT p.name FROM role_permissions rp JOIN permissions p ON p.id = rp.permission_id WHERE rp.role_id = u.role_id)")
	}
	query, args, err := sb.ToSql()
	if err != nil {
		return nil, fmt.Errorf("build query: %w", err)
	}

	var u user.User
	var rID *int64
	var rName, rDesc *string
	var rCreatedAt, rUpdatedAt *time.Time

	dest := []any{
		&u.ID, &u.Email, &u.Name, &u.PasswordHash, &u.RoleID,
		&u.CreatedAt, &u.UpdatedAt, &u.AreaAssignment, &u.ChurchName, &u.Status, &u.TokensValidAfter, &u.DeletedAt, &u.AvatarURL,
		&rID, &rName, &rDesc, &rCreatedAt, &rUpdatedAt,
	}
	if perms != nil {
		dest = append(dest, perms)
	}
	err = r.db.Querier(ctx).QueryRow(ctx, query, args...).Scan(dest...)
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

// LoginLocked reports whether userID is locked out from ip.
func (r *store) LoginLocked(ctx context.Context, userID int64, ip string) (bool, error) {
	var locked bool
	err := r.db.Querier(ctx).QueryRow(ctx,
		`SELECT EXISTS (SELECT 1 FROM login_failures WHERE user_id = $1 AND ip = $2 AND locked_until > NOW())`,
		userID, ip).Scan(&locked)
	if err != nil {
		return false, fmt.Errorf("login locked: %w", err)
	}
	return locked, nil
}

// RecordFailedLogin counts one failure for userID from ip and locks that pair once
// the count reaches maxAttempts. A missing user is database.ErrNotFound.
func (r *store) RecordFailedLogin(ctx context.Context, userID int64, ip string, maxAttempts int, lockDuration time.Duration) error {
	_, err := r.db.Querier(ctx).Exec(ctx, `
		INSERT INTO login_failures AS f (user_id, ip, attempts, locked_until)
		VALUES ($1, $2, 1, CASE WHEN 1 >= $3 THEN NOW() + $4::interval END)
		ON CONFLICT (user_id, ip) DO UPDATE SET
			attempts = f.attempts + 1,
			locked_until = CASE WHEN f.attempts + 1 >= $3 THEN NOW() + $4::interval ELSE f.locked_until END,
			updated_at = NOW()`,
		userID, ip, maxAttempts, lockDuration.String())
	if database.IsForeignKeyViolation(err) {
		return fmt.Errorf("users: %w", database.ErrNotFound)
	}
	if err != nil {
		return fmt.Errorf("record failed login: %w", err)
	}
	return nil
}

// ResetLoginAttempts clears the failures of userID from ip, after a right password from it.
func (r *store) ResetLoginAttempts(ctx context.Context, userID int64, ip string) error {
	if _, err := r.db.Querier(ctx).Exec(ctx, `DELETE FROM login_failures WHERE user_id = $1 AND ip = $2`, userID, ip); err != nil {
		return fmt.Errorf("reset login attempts: %w", err)
	}
	return nil
}

// ClearLockout clears the failures of userID from every IP.
func (r *store) ClearLockout(ctx context.Context, userID int64) error {
	if _, err := r.db.Querier(ctx).Exec(ctx, `DELETE FROM login_failures WHERE user_id = $1`, userID); err != nil {
		return fmt.Errorf("clear lockout: %w", err)
	}
	return nil
}

// UpdateStatus sets a user's status (in a transaction ctx, so the audit row and the
// session revoke share it) and returns the updated user. A missing user is database.ErrNotFound.
func (r *store) UpdateStatus(ctx context.Context, id int64, status string) (*user.User, error) {
	return r.setColumn(ctx, id, "status", status)
}

// setColumn sets one column (a constant name, never user input) and returns the
// updated user. A missing user is database.ErrNotFound.
func (r *store) setColumn(ctx context.Context, id int64, column string, value any) (*user.User, error) {
	query, args, err := r.db.Builder.
		Update("users").
		Set(column, value).
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

// SoftDelete marks a user deleted (D-A11) in the transaction in ctx and returns the
// updated user. The status is not touched. A missing user is database.ErrNotFound.
func (r *store) SoftDelete(ctx context.Context, id, deletedBy int64, reason string) (*user.User, error) {
	return r.setDeleted(ctx, id, sq.Expr("NOW()"), &deletedBy, nullable(reason))
}

// Restore clears a user's deleted mark (D-A11); the history stays in the audit rows.
// A missing user is database.ErrNotFound.
func (r *store) Restore(ctx context.Context, id int64) (*user.User, error) {
	return r.setDeleted(ctx, id, nil, nil, nil)
}

func (r *store) setDeleted(ctx context.Context, id int64, at any, by *int64, reason *string) (*user.User, error) {
	query, args, err := r.db.Builder.
		Update("users").
		Set("deleted_at", at).
		Set("deleted_by", by).
		Set("deleted_reason", reason).
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
		 WHERE r.name = 'admin' AND u.status = 'active' AND u.deleted_at IS NULL ORDER BY u.id FOR UPDATE OF u`)
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
