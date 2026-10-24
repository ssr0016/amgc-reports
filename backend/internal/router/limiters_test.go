package router

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"

	"github.com/labstack/echo/v4"
	"golang.org/x/time/rate"

	"github.com/ssr0016/pastoral-reports/backend/internal/apperror"
	authrest "github.com/ssr0016/pastoral-reports/backend/internal/auth/rest"
	ourmiddleware "github.com/ssr0016/pastoral-reports/backend/internal/middleware"
)

// globalServer mounts the real routes behind a global per-IP limiter of the
// given burst (practically no refill), plus a root /health route registered
// the way main does, outside Setup.
func globalServer(burst int) *echo.Echo {
	e := echo.New()
	e.HTTPErrorHandler = apperror.ErrorHandler
	e.GET("/health", func(c echo.Context) error { return c.String(http.StatusOK, "ok") })
	Setup(e, noAuthn, nil,
		authrest.NewAuthHandler(nil, nil), nil, nil, nil, nil, nil,
		authrest.NewVerificationHandler(nil), authrest.NewPasswordResetHandler(nil),
		nil, nil, nil, Limiters{Global: ourmiddleware.RateLimit(rate.Limit(0.0001), burst), Read: noLimit, Write: noLimit},
	)
	return e
}

// fromIP sends GET path as the given client address and returns the response.
func fromIP(e *echo.Echo, path, ip string) *httptest.ResponseRecorder {
	req := httptest.NewRequestWithContext(context.Background(), http.MethodGet, path, nil)
	req.RemoteAddr = ip + ":1234"
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)
	return rec
}

func TestGlobalLimit_429WithRetryAfter(t *testing.T) {
	e := globalServer(2)
	for i := 0; i < 2; i++ {
		if got := fromIP(e, "/api/v1/auth/csrf", "203.0.113.1").Code; got != http.StatusOK {
			t.Fatalf("request #%d within burst: status = %d, want 200", i, got)
		}
	}
	rec := fromIP(e, "/api/v1/auth/csrf", "203.0.113.1")
	if rec.Code != http.StatusTooManyRequests {
		t.Fatalf("over burst: status = %d, want 429", rec.Code)
	}
	if secs, err := strconv.Atoi(rec.Header().Get(echo.HeaderRetryAfter)); err != nil || secs < 1 {
		t.Errorf("Retry-After = %q, want a positive number of seconds", rec.Header().Get(echo.HeaderRetryAfter))
	}
}

func TestGlobalLimit_EachIPHasItsOwnBucket(t *testing.T) {
	e := globalServer(1)
	fromIP(e, "/api/v1/auth/csrf", "203.0.113.1")
	if got := fromIP(e, "/api/v1/auth/csrf", "203.0.113.1").Code; got != http.StatusTooManyRequests {
		t.Fatalf("first IP over burst: status = %d, want 429", got)
	}
	if got := fromIP(e, "/api/v1/auth/csrf", "198.51.100.7").Code; got != http.StatusOK {
		t.Errorf("second IP: status = %d, want 200", got)
	}
}

// The platform healthcheck and Prometheus must never see a 429, however many
// requests arrive from one address.
func TestGlobalLimit_SystemRoutesAreNotLimited(t *testing.T) {
	e := globalServer(1)
	for i := 0; i < 20; i++ {
		if got := fromIP(e, "/health", "203.0.113.1").Code; got != http.StatusOK {
			t.Fatalf("/health #%d: status = %d, want 200", i, got)
		}
	}
}

func readLimitServer(burst int, perms ...string) *echo.Echo {
	return reportsServerWith(func() Limiters {
		return Limiters{Global: noLimit, Read: ourmiddleware.RateLimitByUser(rate.Limit(0.0001), burst), Write: noLimit}
	}, perms...)
}

// List and detail share one per-user read bucket.
func TestReadLimit_SharedAcrossGetRoutesAndPerUser(t *testing.T) {
	e := readLimitServer(2, "reports:read")
	get := func(path, user string) int { return reportRequestAs(e, http.MethodGet, path, "", user) }

	if got := get("/api/v1/reports", "1"); got != http.StatusOK {
		t.Fatalf("list: status = %d, want 200", got)
	}
	if got := get("/api/v1/reports/1", "1"); got != http.StatusOK {
		t.Fatalf("detail: status = %d, want 200", got)
	}
	if got := get("/api/v1/reports", "1"); got != http.StatusTooManyRequests {
		t.Errorf("list over burst: status = %d, want 429", got)
	}
	if got := get("/api/v1/reports/1", "1"); got != http.StatusTooManyRequests {
		t.Errorf("detail over burst: status = %d, want 429", got)
	}
	if got := get("/api/v1/reports", "2"); got != http.StatusOK {
		t.Errorf("other user on the same IP: status = %d, want 200", got)
	}
}

// A caller without reports:read gets 403 and must not use tokens.
func TestReadLimit_DeniedRequestsDoNotConsumeTokens(t *testing.T) {
	e := readLimitServer(1, "reports:write")
	for i := 0; i < 5; i++ {
		if got := reportRequest(e, http.MethodGet, "/api/v1/reports", "", true); got != http.StatusForbidden {
			t.Fatalf("GET #%d: status = %d, want 403 (never 429)", i, got)
		}
	}
}
