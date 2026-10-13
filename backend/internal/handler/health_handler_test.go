package handler

import (
	"bytes"
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/labstack/echo/v4"

	"github.com/ssr0016/pastoral-reports/backend/internal/logger"
)

// An unreachable database must not be described to whoever asked. The detail
// belongs in the server log.
func TestReadiness_FailureDoesNotLeakError(t *testing.T) {
	// Port 1 refuses at once; pgxpool.New does not connect until first use.
	pool, err := pgxpool.New(context.Background(), "postgres://leaky:secret@127.0.0.1:1/appdb?connect_timeout=1")
	if err != nil {
		t.Fatalf("pgxpool.New: %v", err)
	}
	t.Cleanup(pool.Close)

	var logs bytes.Buffer
	ctx := logger.WithContext(context.Background(), slog.New(slog.NewJSONHandler(&logs, nil)))

	e := echo.New()
	e.GET("/ready", NewHealthHandler(pool).Readiness)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, httptest.NewRequestWithContext(ctx, http.MethodGet, "/ready", nil))

	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusServiceUnavailable)
	}
	var body map[string]string
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode %q: %v", rec.Body.String(), err)
	}
	if _, ok := body["error"]; ok {
		t.Errorf("body has an error field: %v", body)
	}
	for _, leak := range []string{"127.0.0.1", "refused", "dial", "leaky", "appdb"} {
		if strings.Contains(rec.Body.String(), leak) {
			t.Errorf("body %q leaks %q", rec.Body.String(), leak)
		}
	}
	if !strings.Contains(logs.String(), "127.0.0.1") {
		t.Errorf("the underlying error was not logged server-side: %q", logs.String())
	}
}
