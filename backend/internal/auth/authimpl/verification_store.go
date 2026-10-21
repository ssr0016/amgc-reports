package authimpl

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"errors"
	"fmt"
	"time"

	sq "github.com/Masterminds/squirrel"
	"github.com/jackc/pgx/v5"

	"github.com/ssr0016/pastoral-reports/backend/internal/auth"
	"github.com/ssr0016/pastoral-reports/backend/internal/database"
)

const TokenLifetime = 24 * time.Hour

type verificationStore struct{ db *database.DB }

// NewVerificationStore returns the auth.VerificationStore over db.
func NewVerificationStore(db *database.DB) auth.VerificationStore {
	return &verificationStore{db: db}
}

const verificationColumns = "id, user_id, token, expires_at, used_at, created_at"

func scanToken(row pgx.Row) (*auth.VerificationToken, error) {
	var t auth.VerificationToken
	err := row.Scan(&t.ID, &t.UserID, &t.Token, &t.ExpiresAt, &t.UsedAt, &t.CreatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("scan token: %w", err)
	}
	return &t, nil
}

func (r *verificationStore) Create(ctx context.Context, userID int64) (*auth.VerificationToken, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return nil, fmt.Errorf("generate token: %w", err)
	}
	token := base64.URLEncoding.EncodeToString(b)

	expiresAt := time.Now().Add(TokenLifetime)

	query, args, err := r.db.Builder.
		Insert("verification_tokens").
		Columns("user_id", "token", "expires_at").
		Values(userID, token, expiresAt).
		Suffix("RETURNING " + verificationColumns).
		ToSql()
	if err != nil {
		return nil, fmt.Errorf("build query: %w", err)
	}

	return scanToken(r.db.Pool.QueryRow(ctx, query, args...))
}

func (r *verificationStore) GetByToken(ctx context.Context, token string) (*auth.VerificationToken, error) {
	query, args, err := r.db.Builder.
		Select(verificationColumns).
		From("verification_tokens").
		Where(sq.Eq{"token": token}).
		ToSql()
	if err != nil {
		return nil, fmt.Errorf("build query: %w", err)
	}

	return scanToken(r.db.Pool.QueryRow(ctx, query, args...))
}

// MarkUsed claims the token. The claim is atomic: only the first caller finds
// used_at unset, any later or concurrent caller gets database.ErrNotFound (wrapped), so
// a token cannot be used twice.
func (r *verificationStore) MarkUsed(ctx context.Context, id int64) error {
	query, args, err := r.db.Builder.
		Update("verification_tokens").
		Set("used_at", sq.Expr("NOW()")).
		Where(sq.Eq{"id": id}).
		Where(sq.Eq{"used_at": nil}).
		ToSql()
	if err != nil {
		return fmt.Errorf("build query: %w", err)
	}

	result, err := r.db.Pool.Exec(ctx, query, args...)
	if err != nil {
		return fmt.Errorf("mark used: %w", err)
	}
	if result.RowsAffected() == 0 {
		return fmt.Errorf("verification_tokens: %w", database.ErrNotFound)
	}
	return nil
}

func (r *verificationStore) DeleteExpired(ctx context.Context) (int64, error) {
	query, args, err := r.db.Builder.
		Delete("verification_tokens").
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
