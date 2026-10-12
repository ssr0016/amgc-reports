package session

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/alexedwards/scs/pgxstore"
	"github.com/alexedwards/scs/v2"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// userIDKey is the session value Login stores the user under.
const userIDKey = "user_id"

// Store is the scs session store over the sessions table. It extends
// pgxstore in two ways:
//
//   - every committed session records the user it belongs to (user_id), so
//     all of a user's sessions can be revoked together;
//   - a revoked session (revoked_at set) is never found, and a late commit
//     cannot bring it back. Every request re-commits its session because of
//     the idle timeout, so a request that loaded the session before the
//     revocation would otherwise upsert it again.
//
// Revoking is a plain UPDATE of revoked_at (see UserRepo.UpdatePassword).
// pgxstore's cleanup still removes the rows once they expire.
type Store struct {
	*pgxstore.PostgresStore
	pool  *pgxpool.Pool
	codec scs.Codec
}

// NewStore returns a Store over the sessions table. codec must be the session
// manager's codec, so the store can read user_id out of the session data.
func NewStore(pool *pgxpool.Pool, codec scs.Codec) *Store {
	return &Store{PostgresStore: pgxstore.New(pool), pool: pool, codec: codec}
}

// FindCtx returns the data of a live session; expired and revoked sessions
// are not found.
func (s *Store) FindCtx(ctx context.Context, token string) ([]byte, bool, error) {
	var b []byte
	err := s.pool.QueryRow(ctx,
		`SELECT data FROM sessions WHERE token = $1 AND current_timestamp < expiry AND revoked_at IS NULL`,
		token).Scan(&b)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, false, nil
	}
	if err != nil {
		return nil, false, err
	}
	return b, true, nil
}

// CommitCtx saves the session, linking it to the user in its data. A session
// that has been revoked is left as it is.
func (s *Store) CommitCtx(ctx context.Context, token string, b []byte, expiry time.Time) error {
	_, values, err := s.codec.Decode(b)
	if err != nil {
		return fmt.Errorf("decode session: %w", err)
	}
	var userID *int64
	if id, ok := values[userIDKey].(int64); ok {
		userID = &id
	}

	_, err = s.pool.Exec(ctx,
		`INSERT INTO sessions (token, data, expiry, user_id) VALUES ($1, $2, $3, $4)
		 ON CONFLICT (token) DO UPDATE
		 SET data = EXCLUDED.data, expiry = EXCLUDED.expiry, user_id = EXCLUDED.user_id
		 WHERE sessions.revoked_at IS NULL`,
		token, b, expiry, userID)
	return err
}

// AllCtx returns the live sessions; expired and revoked ones are left out.
func (s *Store) AllCtx(ctx context.Context) (map[string][]byte, error) {
	rows, err := s.pool.Query(ctx,
		`SELECT token, data FROM sessions WHERE current_timestamp < expiry AND revoked_at IS NULL`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	sessions := make(map[string][]byte)
	for rows.Next() {
		var (
			token string
			data  []byte
		)
		if err := rows.Scan(&token, &data); err != nil {
			return nil, err
		}
		sessions[token] = data
	}
	return sessions, rows.Err()
}

// The context-free methods of pgxstore panic. scs only calls the Ctx variants,
// but these keep a direct caller on the same rules instead of a panic.

func (s *Store) Find(token string) ([]byte, bool, error) {
	return s.FindCtx(context.Background(), token)
}

func (s *Store) Commit(token string, b []byte, expiry time.Time) error {
	return s.CommitCtx(context.Background(), token, b, expiry)
}

func (s *Store) All() (map[string][]byte, error) {
	return s.AllCtx(context.Background())
}
