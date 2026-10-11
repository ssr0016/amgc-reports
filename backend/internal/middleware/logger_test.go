package middleware

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/labstack/echo/v4"

	"github.com/ssr0016/pastoral-reports/backend/internal/apperror"
)

func TestSlogLogger_LevelAndStatus(t *testing.T) {
	tests := []struct {
		name       string
		handler    echo.HandlerFunc
		wantStatus int
		wantLevel  string
	}{
		{"401 warns", func(echo.Context) error { return apperror.Unauthorized("no") }, http.StatusUnauthorized, "WARN"},
		{"403 warns", func(echo.Context) error { return apperror.Forbidden("no") }, http.StatusForbidden, "WARN"},
		{"422 warns", func(echo.Context) error { return apperror.Validation("bad") }, http.StatusUnprocessableEntity, "WARN"},
		{"429 warns", func(echo.Context) error { return apperror.RateLimit("slow") }, http.StatusTooManyRequests, "WARN"},
		{"500 errors", func(echo.Context) error { return errors.New("boom") }, http.StatusInternalServerError, "ERROR"},
		{"200 infos", func(c echo.Context) error { return c.NoContent(http.StatusOK) }, http.StatusOK, "INFO"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var buf bytes.Buffer
			log := slog.New(slog.NewJSONHandler(&buf, nil))

			e := echo.New()
			e.HTTPErrorHandler = apperror.ErrorHandler
			e.Use(SlogLogger(log))
			e.GET("/l", tt.handler)

			rec := httptest.NewRecorder()
			e.ServeHTTP(rec, httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/l", nil))

			var entry struct {
				Level  string `json:"level"`
				Status int    `json:"status"`
			}
			if err := json.Unmarshal(buf.Bytes(), &entry); err != nil {
				t.Fatalf("decode log line %q: %v", buf.String(), err)
			}
			if entry.Level != tt.wantLevel {
				t.Errorf("level = %s, want %s", entry.Level, tt.wantLevel)
			}
			if entry.Status != tt.wantStatus {
				t.Errorf("logged status = %d, want %d", entry.Status, tt.wantStatus)
			}
		})
	}
}

func TestSlogLogger_RedactsSensitiveQueryParams(t *testing.T) {
	const secret = "SECRET"

	tests := []struct {
		name    string
		method  string
		target  string
		wantURI string
		secret  string // value that must not appear anywhere in the log line
	}{
		{"1 token redacted", http.MethodGet, "/auth/verify-email?token=SECRET",
			"/auth/verify-email?token=[REDACTED]", secret},
		{"2 other params untouched", http.MethodGet, "/users?email=user@example.com",
			"/users?email=user@example.com", ""},
		{"3 token among other params", http.MethodGet, "/users?email=x&token=SECRET&page=2",
			"/users?email=x&token=[REDACTED]&page=2", secret},
		{"4 bad escape marks only that pair", http.MethodGet, "/path?malformed=%zz",
			"/path?[MALFORMED]", ""},
		{"5 no query", http.MethodGet, "/path",
			"/path", ""},
		{"6 post without query", http.MethodPost, "/api/v1/auth/login",
			"/api/v1/auth/login", ""},
		{"7 duplicate token params", http.MethodGet, "/auth/verify-email?token=SECRET&token=SECRET2",
			"/auth/verify-email?token=[REDACTED]&token=[REDACTED]", secret},
		{"8 percent-encoded key", http.MethodGet, "/auth/verify-email?%74oken=SECRET",
			"/auth/verify-email?%74oken=[REDACTED]", secret},
		{"9 case-insensitive key", http.MethodGet, "/auth/verify-email?TOKEN=SECRET",
			"/auth/verify-email?TOKEN=[REDACTED]", secret},
		{"10 malformed sibling is marked, token redacted", http.MethodGet, "/auth/verify-email?token=SECRET&bad=%zz",
			"/auth/verify-email?token=[REDACTED]&[MALFORMED]", secret},
		{"11 empty token value", http.MethodGet, "/auth/verify-email?token=",
			"/auth/verify-email?token=[REDACTED]", ""},
		{"12 key without value left as is", http.MethodGet, "/auth/verify-email?token",
			"/auth/verify-email?token", ""},
		{"13 empty segments preserved", http.MethodGet, "/auth/verify-email?&token=SECRET&",
			"/auth/verify-email?&token=[REDACTED]&", secret},
		{"14 semicolon pair is marked whole", http.MethodGet, "/auth/verify-email?a=1;token=SECRET",
			"/auth/verify-email?[MALFORMED]", secret},
		{"15 valid params survive a malformed pair", http.MethodGet, "/users?email=a@b.com&x=%zz",
			"/users?email=a@b.com&[MALFORMED]", ""},
		{"16 only the semicolon pair is marked", http.MethodGet, "/users?a=1&b=2;token=SECRET&c=3",
			"/users?a=1&[MALFORMED]&c=3", secret},
		{"17 undecodable key is marked", http.MethodGet, "/users?%zz=1&page=2",
			"/users?[MALFORMED]&page=2", ""},
		{"18 sensitive key with bad escape is marked", http.MethodGet, "/auth/verify-email?token=SEC%zzRET",
			"/auth/verify-email?[MALFORMED]", "SEC"},
		{"19 values are never decoded", http.MethodGet, "/users?q=%41%20b",
			"/users?q=%41%20b", ""},
		{"20 truncated escape is marked", http.MethodGet, "/users?x=%4&page=2",
			"/users?[MALFORMED]&page=2", ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var buf bytes.Buffer
			log := slog.New(slog.NewJSONHandler(&buf, &slog.HandlerOptions{Level: slog.LevelDebug}))

			e := echo.New()
			e.HTTPErrorHandler = apperror.ErrorHandler
			e.Use(SlogLogger(log))
			e.Any("/*", func(c echo.Context) error { return c.NoContent(http.StatusOK) })

			rec := httptest.NewRecorder()
			e.ServeHTTP(rec, httptest.NewRequestWithContext(context.Background(), tt.method, tt.target, nil))

			var entry struct {
				URI string `json:"uri"`
			}
			if err := json.Unmarshal(buf.Bytes(), &entry); err != nil {
				t.Fatalf("decode log line %q: %v", buf.String(), err)
			}
			if entry.URI != tt.wantURI {
				t.Errorf("uri = %q, want %q", entry.URI, tt.wantURI)
			}
			if tt.secret != "" && strings.Contains(buf.String(), tt.secret) {
				t.Errorf("secret %q leaked into log line: %s", tt.secret, buf.String())
			}
		})
	}
}

func TestSlogLogger_LogsNormalizedClientIP(t *testing.T) {
	tests := []struct {
		name   string
		header string
		want   string
	}{
		{"valid forwarded IP", "203.0.113.7, 10.0.0.1", "203.0.113.7"},
		{"huge junk falls back to the connection", strings.Repeat("x", 50<<10), "192.0.2.1"},
		{"padding after a valid IP is not logged", "203.0.113.7, " + strings.Repeat("x", 50<<10), "203.0.113.7"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var buf bytes.Buffer
			log := slog.New(slog.NewJSONHandler(&buf, nil))

			e := echo.New()
			e.Use(SlogLogger(log))
			e.GET("/x", func(c echo.Context) error { return c.NoContent(http.StatusOK) })

			req := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/x", nil)
			req.Header.Set(echo.HeaderXForwardedFor, tt.header)
			e.ServeHTTP(httptest.NewRecorder(), req)

			var entry struct {
				RemoteIP string `json:"remote_ip"`
			}
			if err := json.Unmarshal(buf.Bytes(), &entry); err != nil {
				t.Fatalf("decode log line: %v", err)
			}
			if entry.RemoteIP != tt.want {
				t.Errorf("remote_ip is %d bytes (%.40q), want %q", len(entry.RemoteIP), entry.RemoteIP, tt.want)
			}
			if buf.Len() > 2<<10 {
				t.Errorf("log line is %d bytes; an attacker-sized header leaked into it", buf.Len())
			}
		})
	}
}

func TestSlogLogger_LogsPeerIPSeparatelyFromClientIP(t *testing.T) {
	tests := []struct {
		name         string
		remoteAddr   string
		forwardedFor string
		wantRemote   string
		wantPeer     string
	}{
		{"no forwarding header", "100.64.0.7:4000", "", "100.64.0.7", "100.64.0.7"},
		{"forwarded client behind a proxy", "100.64.0.7:4000", "203.0.113.7", "203.0.113.7", "100.64.0.7"},
		{"junk header: client falls back, peer unchanged", "100.64.0.7:4000", "not-an-ip", "100.64.0.7", "100.64.0.7"},
		{"huge header does not reach either field", "100.64.0.7:4000", strings.Repeat("x", 50<<10), "100.64.0.7", "100.64.0.7"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var buf bytes.Buffer
			log := slog.New(slog.NewJSONHandler(&buf, nil))

			e := echo.New()
			e.Use(SlogLogger(log))
			e.GET("/x", func(c echo.Context) error { return c.NoContent(http.StatusOK) })

			req := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/x", nil)
			req.RemoteAddr = tt.remoteAddr
			if tt.forwardedFor != "" {
				req.Header.Set(echo.HeaderXForwardedFor, tt.forwardedFor)
			}
			e.ServeHTTP(httptest.NewRecorder(), req)

			var entry struct {
				RemoteIP string `json:"remote_ip"`
				PeerIP   string `json:"peer_ip"`
			}
			if err := json.Unmarshal(buf.Bytes(), &entry); err != nil {
				t.Fatalf("decode log line: %v", err)
			}
			if entry.RemoteIP != tt.wantRemote {
				t.Errorf("remote_ip = %q, want %q", entry.RemoteIP, tt.wantRemote)
			}
			if entry.PeerIP != tt.wantPeer {
				t.Errorf("peer_ip = %q, want %q", entry.PeerIP, tt.wantPeer)
			}
			if buf.Len() > 2<<10 {
				t.Errorf("log line is %d bytes", buf.Len())
			}
		})
	}
}
