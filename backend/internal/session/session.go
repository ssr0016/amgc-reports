package session

import (
	"context"
	"net/http"
	"os"
	"strconv"
	"time"

	"github.com/alexedwards/scs/pgxstore"
	"github.com/alexedwards/scs/v2"
	"github.com/jackc/pgx/v5/pgxpool"
)

func New(pool *pgxpool.Pool) *scs.SessionManager {
	sm := scs.New()
	sm.Store = pgxstore.New(pool)
	sm.Cookie.Name = getEnv("SESSION_COOKIE_NAME", "app_session")
	sm.Cookie.HttpOnly = true
	sm.Cookie.Secure = os.Getenv("APP_ENV") == "production"
	sm.Cookie.SameSite = http.SameSiteLaxMode
	sm.Cookie.Path = "/"
	lifetime, _ := strconv.Atoi(getEnv("SESSION_LIFETIME", "86400"))
	sm.Lifetime = time.Duration(lifetime) * time.Second
	sm.IdleTimeout = 30 * time.Minute
	sm.ErrorFunc = func(w http.ResponseWriter, r *http.Request, err error) {
		// log error
	}
	return sm
}

func getEnv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

// Helper para mag-load ng session sa Echo context
func Get[T any](ctx context.Context, sm *scs.SessionManager, key string) T {
	var zero T
	v := sm.Get(ctx, key)
	if v == nil {
		return zero
	}
	if t, ok := v.(T); ok {
		return t
	}
	return zero
}

func GetInt64(ctx context.Context, sm *scs.SessionManager, key string) int64 {
	return sm.GetInt64(ctx, key)
}

func Put(ctx context.Context, sm *scs.SessionManager, key string, value any) {
	sm.Put(ctx, key, value)
}

func Destroy(ctx context.Context, sm *scs.SessionManager) error {
	return sm.Destroy(ctx)
}
