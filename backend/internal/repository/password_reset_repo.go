package repository

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"errors"
	"fmt"
	"time"

	sq "github.com/Masterminds/squirrel"
	"github.com/jackc/pgx/v5"

	"github.com/ssr0016/template/internal/database"
	"github.com/ssr0016/template/internal/model"
)

const PasswordResetLifetime = 1 * time.Hour

type PasswordResetRepo struct{ db *database.DB }

func NewPasswordResetRepo(db *database.DB) *PasswordResetRepo {
	return &PasswordResetRepo{db: db}
}

const passwordResetColumns = "id, user_id, token, expires_at, used_at, created_at" // #nosec G101 - not credentials, column names

func scanPasswordReset(row pgx.Row) (*model.PasswordReset, error) {
	var p model.PasswordReset
	err := row.Scan(&p.ID, &p.UserID, &p.Token, &p.ExpiresAt, &p.UsedAt, &p.CreatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("scan password reset: %w", err)
	}
	return &p, nil
}

func (r *PasswordResetRepo) Create(ctx context.Context, userID int64) (*model.PasswordReset, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return nil, fmt.Errorf("generate token: %w", err)
	}
	token := base64.URLEncoding.EncodeToString(b)

	expiresAt := time.Now().Add(PasswordResetLifetime)

	query, args, err := r.db.Builder.
		Insert("password_resets").
		Columns("user_id", "token", "expires_at").
		Values(userID, token, expiresAt).
		Suffix("RETURNING " + passwordResetColumns).
		ToSql()
	if err != nil {
		return nil, fmt.Errorf("build query: %w", err)
	}

	return scanPasswordReset(r.db.Pool.QueryRow(ctx, query, args...))
}

func (r *PasswordResetRepo) GetByToken(ctx context.Context, token string) (*model.PasswordReset, error) {
	query, args, err := r.db.Builder.
		Select(passwordResetColumns).
		From("password_resets").
		Where(sq.Eq{"token": token}).
		ToSql()
	if err != nil {
		return nil, fmt.Errorf("build query: %w", err)
	}

	return scanPasswordReset(r.db.Pool.QueryRow(ctx, query, args...))
}

func (r *PasswordResetRepo) MarkUsed(ctx context.Context, id int64) error {
	query, args, err := r.db.Builder.
		Update("password_resets").
		Set("used_at", sq.Expr("NOW()")).
		Where(sq.Eq{"id": id}).
		ToSql()
	if err != nil {
		return fmt.Errorf("build query: %w", err)
	}

	_, err = r.db.Pool.Exec(ctx, query, args...)
	if err != nil {
		return fmt.Errorf("mark used: %w", err)
	}
	return nil
}

func (r *PasswordResetRepo) DeleteExpired(ctx context.Context) (int64, error) {
	query, args, err := r.db.Builder.
		Delete("password_resets").
		Where(sq.Lt{"expires_at": sq.Expr("NOW()")}).
		ToSql()
	if err != nil {
		return 0, fmt.Errorf("build query: %w", err)
	}

	result, err := r.db.Pool.Exec(ctx, query, args...)
	if err != nil {
		return 0, fmt.Errorf("delete expired: %w", err)
	}
	return result.RowsAffected(), nil
}
