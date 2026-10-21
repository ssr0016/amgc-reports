package rest

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/labstack/echo/v4"

	"github.com/ssr0016/pastoral-reports/backend/internal/apperror"
	ourmiddleware "github.com/ssr0016/pastoral-reports/backend/internal/middleware"
)

func newCSRFTestServer() *echo.Echo {
	e := echo.New()
	e.HTTPErrorHandler = apperror.ErrorHandler
	e.Use(ourmiddleware.CSRFProtection())
	e.GET("/csrf", CSRFToken)
	e.POST("/write", func(c echo.Context) error { return c.NoContent(http.StatusNoContent) })
	return e
}

func TestCSRFToken_MatchesCookieAndAllowsWrite(t *testing.T) {
	e := newCSRFTestServer()

	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/csrf", nil))
	if rec.Code != http.StatusOK {
		t.Fatalf("GET /csrf = %d, want 200", rec.Code)
	}

	var cookieVal string
	for _, ck := range rec.Result().Cookies() {
		if ck.Name == "csrf_token" {
			cookieVal = ck.Value
		}
	}
	if cookieVal == "" {
		t.Fatal("csrf_token cookie not set")
	}
	if !strings.Contains(rec.Body.String(), `"csrf_token":"`+cookieVal+`"`) {
		t.Fatalf("body %q does not contain cookie token %q", rec.Body.String(), cookieVal)
	}

	// Using the body token as the header must pass the CSRF check.
	req := httptest.NewRequestWithContext(context.Background(), http.MethodPost, "/write", nil)
	req.AddCookie(&http.Cookie{Name: "csrf_token", Value: cookieVal})
	req.Header.Set("X-CSRF-Token", cookieVal)
	rec = httptest.NewRecorder()
	e.ServeHTTP(rec, req)
	if rec.Code != http.StatusNoContent {
		t.Fatalf("POST with token = %d, want 204", rec.Code)
	}

	// Without the header it must be rejected.
	req = httptest.NewRequestWithContext(context.Background(), http.MethodPost, "/write", nil)
	req.AddCookie(&http.Cookie{Name: "csrf_token", Value: cookieVal})
	rec = httptest.NewRecorder()
	e.ServeHTTP(rec, req)
	if rec.Code != http.StatusForbidden {
		t.Fatalf("POST without token = %d, want 403", rec.Code)
	}
}

func TestCSRFToken_StableForSameClient(t *testing.T) {
	e := newCSRFTestServer()

	req := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/csrf", nil)
	req.AddCookie(&http.Cookie{Name: "csrf_token", Value: "existing-token"})
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	if !strings.Contains(rec.Body.String(), `"csrf_token":"existing-token"`) {
		t.Fatalf("body = %q, want existing token returned", rec.Body.String())
	}
}
