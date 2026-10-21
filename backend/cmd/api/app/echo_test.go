package app

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/labstack/echo/v4"
	"github.com/prometheus/client_golang/prometheus/testutil"

	"github.com/ssr0016/pastoral-reports/backend/internal/config"
	ourmiddleware "github.com/ssr0016/pastoral-reports/backend/internal/middleware"
	"github.com/ssr0016/pastoral-reports/backend/pkg/metrics"
)

// postOversize sends a POST whose declared Content-Length is over the body
// limit through the full global chain and returns the response status and the
// JSON log lines written meanwhile.
func postOversize(t *testing.T, path string) (int, []map[string]any) {
	t.Helper()

	var buf bytes.Buffer
	e := newEcho(&config.Config{}, newJSONLogger(&buf))
	e.POST(path, func(c echo.Context) error { return c.NoContent(http.StatusOK) })

	body := bytes.NewReader(make([]byte, ourmiddleware.MaxRequestBodyBytes+1))
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, httptest.NewRequestWithContext(context.Background(), http.MethodPost, path, body))

	var lines []map[string]any
	sc := bufio.NewScanner(&buf)
	for sc.Scan() {
		var m map[string]any
		if err := json.Unmarshal(sc.Bytes(), &m); err != nil {
			t.Fatalf("decode log line %q: %v", sc.Text(), err)
		}
		lines = append(lines, m)
	}
	return rec.Code, lines
}

// A request the body limit rejects is still a request: it must show up in the
// access log and in http_requests_total, or abuse with large bodies is
// invisible.
func TestMiddlewareChain_OversizeBodyIsLogged(t *testing.T) {
	code, lines := postOversize(t, "/chain/logged")

	if code != http.StatusRequestEntityTooLarge {
		t.Fatalf("status = %d, want %d", code, http.StatusRequestEntityTooLarge)
	}
	for _, m := range lines {
		if m["status"] == float64(http.StatusRequestEntityTooLarge) {
			if m["level"] != "WARN" {
				t.Errorf("level = %v, want WARN", m["level"])
			}
			return
		}
	}
	t.Errorf("no access log line with status 413 among %d lines: %v", len(lines), lines)
}

func TestMiddlewareChain_OversizeBodyIsCounted(t *testing.T) {
	const path = "/chain/counted"
	counter := metrics.HTTPRequestsTotal.WithLabelValues(http.MethodPost, path, "413")
	before := testutil.ToFloat64(counter)

	code, _ := postOversize(t, path)

	if code != http.StatusRequestEntityTooLarge {
		t.Fatalf("status = %d, want %d", code, http.StatusRequestEntityTooLarge)
	}
	if got := testutil.ToFloat64(counter) - before; got != 1 {
		t.Errorf("http_requests_total{status=413} delta = %v, want 1", got)
	}
}

// The frontend is cross-origin, and a browser hides every response header that
// is not exposed: without this, scripts cannot read Retry-After on a 429.
func TestNewEcho_CORSExposesRetryAfter(t *testing.T) {
	const origin = "http://localhost:3000"
	cfg := &config.Config{}
	cfg.CORS.AllowedOrigins = []string{origin}

	var buf bytes.Buffer
	e := newEcho(cfg, newJSONLogger(&buf))
	e.GET("/cors", func(c echo.Context) error { return c.NoContent(http.StatusOK) })

	req := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/cors", nil)
	req.Header.Set(echo.HeaderOrigin, origin)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	if got := rec.Header().Get(echo.HeaderAccessControlExposeHeaders); got != "Retry-After" {
		t.Errorf("Access-Control-Expose-Headers = %q, want %q", got, "Retry-After")
	}
}

func newJSONLogger(buf *bytes.Buffer) *slog.Logger {
	return slog.New(slog.NewJSONHandler(buf, nil))
}
