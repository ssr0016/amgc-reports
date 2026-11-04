package app

import (
	"bufio"
	"bytes"
	"compress/gzip"
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/getsentry/sentry-go"
	"github.com/labstack/echo/v4"
	"github.com/prometheus/client_golang/prometheus/testutil"

	"github.com/ssr0016/pastoral-reports/backend/internal/apperror"
	"github.com/ssr0016/pastoral-reports/backend/internal/config"
	ourmiddleware "github.com/ssr0016/pastoral-reports/backend/internal/middleware"
	"github.com/ssr0016/pastoral-reports/backend/pkg/metrics"
)

func TestMiddlewareChain_Gzip(t *testing.T) {
	large := strings.Repeat("report data ", 1000)
	for _, tc := range []struct {
		name, path, target, accept, body string
		compressed                       bool
	}{
		{"large JSON", "/api/v1/reports", "/api/v1/reports", "gzip", large, true},
		{"small response", "/small", "/small", "gzip", "ok", false},
		{"without negotiation", "/plain", "/plain", "", large, false},
		{"Swagger asset", "/swagger/*", "/swagger/swagger-ui.css", "gzip", large, false},
		{"Excel download", "/api/v1/admin/reports/:id/excel", "/api/v1/admin/reports/1/excel", "gzip", large, false},
		{"Prometheus handles compression", "/metrics", "/metrics", "gzip", large, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			e := newEcho(&config.Config{}, slog.New(slog.DiscardHandler))
			e.GET(tc.path, func(c echo.Context) error { return c.JSON(http.StatusOK, map[string]string{"data": tc.body}) })
			req := httptest.NewRequestWithContext(t.Context(), http.MethodGet, tc.target, nil)
			req.Header.Set(echo.HeaderAcceptEncoding, tc.accept)
			rec := httptest.NewRecorder()
			e.ServeHTTP(rec, req)
			if got := rec.Header().Get(echo.HeaderContentEncoding) == "gzip"; got != tc.compressed {
				t.Fatalf("compressed = %v, want %v", got, tc.compressed)
			}
			body := rec.Body.Bytes()
			if tc.compressed {
				reader, err := gzip.NewReader(rec.Body)
				if err != nil {
					t.Fatal(err)
				}
				defer func() {
					if err := reader.Close(); err != nil {
						t.Error(err)
					}
				}()
				body, err = io.ReadAll(reader)
				if err != nil {
					t.Fatal(err)
				}
			}
			var decoded map[string]string
			if err := json.Unmarshal(body, &decoded); err != nil {
				t.Fatal(err)
			}
			if rec.Code != http.StatusOK || decoded["data"] != tc.body {
				t.Fatalf("response changed: status %d, data length %d", rec.Code, len(decoded["data"]))
			}
		})
	}
}

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
func TestNewEcho_CORSExposesRetryAfterAndContentDisposition(t *testing.T) {
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

	if got, want := rec.Header().Get(echo.HeaderAccessControlExposeHeaders), "Retry-After,Content-Disposition"; got != want {
		t.Errorf("Access-Control-Expose-Headers = %q, want %q", got, want)
	}
}

func newJSONLogger(buf *bytes.Buffer) *slog.Logger {
	return slog.New(slog.NewJSONHandler(buf, nil))
}

// Each server failure must reach Sentry exactly once: a panic (which Recover
// hands to the error handler) and a returned 5xx once each, a 4xx not at all.
func TestMiddlewareChain_ReportsServerErrorsOnce(t *testing.T) {
	transport := &sentry.MockTransport{}
	client, err := sentry.NewClient(sentry.ClientOptions{Transport: transport})
	if err != nil {
		t.Fatal(err)
	}
	sentry.CurrentHub().BindClient(client)
	t.Cleanup(func() { sentry.CurrentHub().BindClient(nil) })

	e := newEcho(&config.Config{}, slog.New(slog.DiscardHandler))
	e.GET("/panic", func(echo.Context) error { panic("boom") })
	e.GET("/fail", func(echo.Context) error { return apperror.Internal("db down") })
	e.GET("/bad", func(echo.Context) error { return apperror.BadRequest("bad input") })

	for _, tc := range []struct {
		path       string
		wantStatus int
		wantEvents int
	}{
		{"/panic", http.StatusInternalServerError, 1},
		{"/fail", http.StatusInternalServerError, 1},
		{"/bad", http.StatusBadRequest, 0},
	} {
		before := len(transport.Events())
		rec := httptest.NewRecorder()
		e.ServeHTTP(rec, httptest.NewRequestWithContext(context.Background(), http.MethodGet, tc.path, nil))
		if rec.Code != tc.wantStatus {
			t.Errorf("%s: status = %d, want %d", tc.path, rec.Code, tc.wantStatus)
		}
		if got := len(transport.Events()) - before; got != tc.wantEvents {
			t.Errorf("%s: events = %d, want %d", tc.path, got, tc.wantEvents)
		}
	}
}
