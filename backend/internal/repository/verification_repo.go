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

const TokenLifetime = 24 * time.Hour

type VerificationRepo struct{ db *database.DB }

func NewVerificationRepo(db *database.DB) *VerificationRepo {
	return &VerificationRepo{db: db}
}

const verificationColumns = "id, user_id, token, expires_at, used_at, created_at"

func scanToken(row pgx.Row) (*model.VerificationToken, error) {
	var t model.VerificationToken
	err := row.Scan(&t.ID, &t.UserID, &t.Token, &t.ExpiresAt, &t.UsedAt, &t.CreatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("scan token: %w", err)
	}
	return &t, nil
}

func (r *VerificationRepo) Create(ctx context.Context, userID int64) (*model.VerificationToken, error) {
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

func (r *VerificationRepo) GetByToken(ctx context.Context, token string) (*model.VerificationToken, error) {
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

func (r *VerificationRepo) MarkUsed(ctx context.Context, id int64) error {
	query, args, err := r.db.Builder.
		Update("verification_tokens").
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

func (r *VerificationRepo) DeleteExpired(ctx context.Context) (int64, error) {
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
