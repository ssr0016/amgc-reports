package authimpl

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"

	"github.com/ssr0016/pastoral-reports/backend/internal/auth"
	"github.com/ssr0016/pastoral-reports/backend/internal/database"
)

// refreshStore stores refresh tokens by hash. See migration
// 20261021100000_create_refresh_tokens for the model.
type refreshStore struct{ db *database.DB }

// NewRefreshStore returns the auth.RefreshStore over db.
func NewRefreshStore(db *database.DB) auth.RefreshStore { return &refreshStore{db: db} }

func (r *refreshStore) Create(ctx context.Context, t auth.NewRefreshToken) (int64, error) {
	var id int64
	err := r.db.Querier(ctx).QueryRow(ctx,
		`INSERT INTO refresh_tokens (user_id, family_id, token_hash, expires_at, created_at, last_used_at, ip, user_agent)
		 VALUES ($1, $2, $3, $4, $5, $5, NULLIF($6, ''), NULLIF($7, '')) RETURNING id`,
		t.UserID, t.FamilyID, t.Hash, t.ExpiresAt, t.Now, t.IP, t.UserAgent).Scan(&id)
	if err != nil {
		return 0, fmt.Errorf("insert refresh token: %w", err)
	}
	return id, nil
}

func (r *refreshStore) UserIDByHash(ctx context.Context, hash string) (int64, bool, error) {
	var id int64
	err := r.db.Querier(ctx).QueryRow(ctx, `SELECT user_id FROM refresh_tokens WHERE token_hash = $1`, hash).Scan(&id)
	if errors.Is(err, pgx.ErrNoRows) {
		return 0, false, nil
	}
	if err != nil {
		return 0, false, fmt.Errorf("get refresh token owner: %w", err)
	}
	return id, true, nil
}

func (r *refreshStore) GetForUpdate(ctx context.Context, hash string) (*auth.RefreshToken, error) {
	var t auth.RefreshToken
	err := r.db.Querier(ctx).QueryRow(ctx,
		`SELECT id, user_id, family_id, expires_at, last_used_at, revoked_at, replaced_by
		 FROM refresh_tokens WHERE token_hash = $1 FOR UPDATE`, hash).
		Scan(&t.ID, &t.UserID, &t.FamilyID, &t.ExpiresAt, &t.LastUsedAt, &t.RevokedAt, &t.ReplacedBy)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("get refresh token: %w", err)
	}
	return &t, nil
}

func (r *refreshStore) MarkReplaced(ctx context.Context, id, replacedBy int64, at time.Time) error {
	_, err := r.db.Querier(ctx).Exec(ctx,
		`UPDATE refresh_tokens SET revoked_at = $2, replaced_by = $3 WHERE id = $1`, id, at, replacedBy)
	if err != nil {
		return fmt.Errorf("mark refresh token replaced: %w", err)
	}
	return nil
}

func (r *refreshStore) RevokeFamily(ctx context.Context, familyID string, at time.Time) error {
	_, err := r.db.Querier(ctx).Exec(ctx,
		`UPDATE refresh_tokens SET revoked_at = $2 WHERE family_id = $1 AND revoked_at IS NULL`, familyID, at)
	if err != nil {
		return fmt.Errorf("revoke refresh family: %w", err)
	}
	return nil
}
