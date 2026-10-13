package router

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/labstack/echo/v4"

	"github.com/ssr0016/pastoral-reports/backend/internal/apperror"
	"github.com/ssr0016/pastoral-reports/backend/internal/handler"
	"github.com/ssr0016/pastoral-reports/backend/internal/validator"
)

const (
	verifyEmailPath    = "/api/v1/auth/verify-email"
	forgotPasswordPath = "/api/v1/auth/forgot-password"
	resetPasswordPath  = "/api/v1/auth/reset-password"
	loginPath          = "/api/v1/auth/login"
	registerPath       = "/api/v1/auth/register"
)

// newTestServer wires the real routes with services left nil. Every handler
// under test rejects an empty request (400 or 422) before it would touch its
// service, so no database or service is needed to exercise the limiters.
func newTestServer() *echo.Echo {
	e := echo.New()
	e.Validator = validator.New()
	e.HTTPErrorHandler = apperror.ErrorHandler

	Setup(e, nil, nil, nil,
		handler.NewAuthHandler(nil, nil),
		nil, nil, nil, nil,
		handler.NewVerificationHandler(nil),
		handler.NewPasswordResetHandler(nil),
	)
	return e
}

// hit sends one empty request and returns the status and the error code.
func hit(t *testing.T, e *echo.Echo, method, path string) (int, string) {
	t.Helper()
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, httptest.NewRequestWithContext(context.Background(), method, path, nil))

	var body struct {
		Error string `json:"error"`
	}
	if rec.Body.Len() > 0 {
		if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
			t.Fatalf("%s %s: decode body %q: %v", method, path, rec.Body.String(), err)
		}
	}
	return rec.Code, body.Error
}

// postJSON sends a JSON body and returns the status and the decoded error body.
func postJSON(t *testing.T, e *echo.Echo, path, body string) (int, apperror.ErrorResponse) {
	t.Helper()
	req := httptest.NewRequestWithContext(context.Background(), http.MethodPost, path, strings.NewReader(body))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	var resp apperror.ErrorResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("POST %s: decode body %q: %v", path, rec.Body.String(), err)
	}
	return rec.Code, resp
}

func TestRegister_EmptyBody_Returns422WithFieldDetails(t *testing.T) {
	code, resp := postJSON(t, newTestServer(), registerPath, `{}`)

	if code != http.StatusUnprocessableEntity {
		t.Fatalf("status = %d, want %d", code, http.StatusUnprocessableEntity)
	}
	for _, field := range []string{"email", "name", "password"} {
		if got := resp.Details[field]; got != "is required" {
			t.Errorf("details[%s] = %q, want %q (details: %v)", field, got, "is required", resp.Details)
		}
	}
}

func TestResetPassword_ShortPassword_ReturnsDetailsKeyedByJSONName(t *testing.T) {
	code, resp := postJSON(t, newTestServer(), resetPasswordPath, `{"token":"abc","new_password":"short"}`)

	if code != http.StatusUnprocessableEntity {
		t.Fatalf("status = %d, want %d", code, http.StatusUnprocessableEntity)
	}
	if got, want := resp.Details["new_password"], "must be at least 8 characters"; got != want {
		t.Errorf("details[new_password] = %q, want %q (details: %v)", got, want, resp.Details)
	}
	if _, ok := resp.Details["token"]; ok {
		t.Errorf("details has token, which was valid: %v", resp.Details)
	}
}

func TestValidationError_KeepsErrorCodeAndStatus(t *testing.T) {
	code, resp := postJSON(t, newTestServer(), forgotPasswordPath, `{}`)

	if code != http.StatusUnprocessableEntity {
		t.Errorf("status = %d, want %d", code, http.StatusUnprocessableEntity)
	}
	if resp.Error != "VALIDATION_ERROR" {
		t.Errorf("error = %q, want VALIDATION_ERROR", resp.Error)
	}
	if resp.Message != "validation failed" {
		t.Errorf("message = %q, want %q (no Go struct names)", resp.Message, "validation failed")
	}
}

func TestTokenRoutes_RateLimited(t *testing.T) {
	tests := []struct {
		name       string
		method     string
		path       string
		wantBefore int // what an unlimited request returns: validation failure
	}{
		{"GET verify-email", http.MethodGet, verifyEmailPath, http.StatusBadRequest},
		{"POST forgot-password", http.MethodPost, forgotPasswordPath, http.StatusUnprocessableEntity},
		{"POST reset-password", http.MethodPost, resetPasswordPath, http.StatusUnprocessableEntity},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			e := newTestServer()

			for i := 1; i <= 3; i++ {
				if code, _ := hit(t, e, tt.method, tt.path); code != tt.wantBefore {
					t.Fatalf("request %d: status = %d, want %d (validation failure before the service)", i, code, tt.wantBefore)
				}
			}

			code, errCode := hit(t, e, tt.method, tt.path)
			if code != http.StatusTooManyRequests {
				t.Errorf("request 4: status = %d, want %d", code, http.StatusTooManyRequests)
			}
			if errCode != "RATE_LIMIT_EXCEEDED" {
				t.Errorf("request 4: error code = %q, want RATE_LIMIT_EXCEEDED", errCode)
			}
		})
	}
}

func TestTokenRoutes_BucketsAreIndependent(t *testing.T) {
	e := newTestServer()

	exhaust := func(method, path string) {
		t.Helper()
		for i := 0; i < 4; i++ {
			hit(t, e, method, path)
		}
		if code, _ := hit(t, e, method, path); code != http.StatusTooManyRequests {
			t.Fatalf("%s %s should be limited after exhausting its bucket, got %d", method, path, code)
		}
	}
	notLimited := func(method, path string) {
		t.Helper()
		if code, _ := hit(t, e, method, path); code == http.StatusTooManyRequests {
			t.Errorf("%s %s was limited by another route's bucket", method, path)
		}
	}

	// Exhausting login must not touch any of the token routes.
	exhaust(http.MethodPost, loginPath)
	notLimited(http.MethodPost, forgotPasswordPath)
	notLimited(http.MethodGet, verifyEmailPath)
	notLimited(http.MethodPost, resetPasswordPath)

	// Exhausting forgot-password must not touch the other two, nor login's
	// own bucket (which stays exhausted).
	exhaust(http.MethodPost, forgotPasswordPath)
	notLimited(http.MethodGet, verifyEmailPath)
	notLimited(http.MethodPost, resetPasswordPath)
	if code, _ := hit(t, e, http.MethodPost, loginPath); code != http.StatusTooManyRequests {
		t.Errorf("login bucket should stay exhausted, got %d", code)
	}
}
