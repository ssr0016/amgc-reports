package router

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"

	"github.com/labstack/echo/v4"

	"github.com/ssr0016/pastoral-reports/backend/internal/apperror"
	"github.com/ssr0016/pastoral-reports/backend/internal/handler"
	"github.com/ssr0016/pastoral-reports/backend/internal/model"
	"github.com/ssr0016/pastoral-reports/backend/internal/repository"
	"github.com/ssr0016/pastoral-reports/backend/internal/service"
	"github.com/ssr0016/pastoral-reports/backend/internal/validator"
)

const (
	verifyEmailPath    = "/api/v1/auth/verify-email"
	forgotPasswordPath = "/api/v1/auth/forgot-password"
	resetPasswordPath  = "/api/v1/auth/reset-password"
	loginPath          = "/api/v1/auth/login"
)

// noLimit is a pass-through for the reports write limiter.
func noLimit(next echo.HandlerFunc) echo.HandlerFunc { return next }

var noLimits = Limiters{Global: noLimit, Read: noLimit, Write: noLimit}

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
		nil, nil, noLimits,
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

// noTokenRepo knows no reset token, so a request that passes validation ends at the
// service as an invalid token (400).
type noTokenRepo struct {
	repository.PasswordResetRepository
}

func (noTokenRepo) GetByToken(context.Context, string) (*model.PasswordReset, error) { return nil, nil }

func resetServer() *echo.Echo {
	e := echo.New()
	e.Validator = validator.New()
	e.HTTPErrorHandler = apperror.ErrorHandler
	svc := service.NewPasswordResetService(repository.NewMockUserRepo(), noTokenRepo{}, slog.New(slog.DiscardHandler))
	Setup(e, nil, nil, nil,
		handler.NewAuthHandler(nil, nil),
		nil, nil, nil, nil,
		handler.NewVerificationHandler(nil),
		handler.NewPasswordResetHandler(svc),
		nil, nil, noLimits,
	)
	return e
}

const strongPassword = "X7kP9mQ2vL8nR4tY6wB3zC5dF1gH0jK"

// Reset applies the same password rules as change-password and admin user creation.
func TestResetPassword_PasswordRules(t *testing.T) {
	tests := []struct {
		name     string
		password string
		status   int
		detail   string
	}{
		{"too short", "short", http.StatusUnprocessableEntity, "is too weak; use a longer, more varied password"},
		{"eight weak characters", "aaaaaaaa", http.StatusUnprocessableEntity, "is too weak; use a longer, more varied password"},
		{"strong but over 72 bytes", strongPassword + strings.Repeat("é", 21), http.StatusUnprocessableEntity, "must be at most 72 bytes"},
		{"strong passes validation and fails on the token", strongPassword, http.StatusBadRequest, ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			code, resp := postJSON(t, resetServer(), resetPasswordPath,
				`{"token":"abc","new_password":"`+tt.password+`","confirm_password":"`+tt.password+`"}`)
			if code != tt.status {
				t.Fatalf("status = %d, want %d (body: %+v)", code, tt.status, resp)
			}
			if got := resp.Details["new_password"]; got != tt.detail {
				t.Errorf("details[new_password] = %q, want %q", got, tt.detail)
			}
			if _, ok := resp.Details["token"]; ok {
				t.Errorf("details has token, which was valid: %v", resp.Details)
			}
		})
	}
}

// The confirmation is checked before the token is looked up.
func TestResetPassword_ConfirmPassword(t *testing.T) {
	tests := []struct {
		name, confirm string
		status        int
		detail        string
	}{
		{"missing", "", http.StatusUnprocessableEntity, "is required"},
		{"different", strongPassword + "x", http.StatusUnprocessableEntity, "does not match"},
		{"case-only difference", strings.ToLower(strongPassword), http.StatusUnprocessableEntity, "does not match"},
		{"same passes validation and fails on the token", strongPassword, http.StatusBadRequest, ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			body := `{"token":"abc","new_password":"` + strongPassword + `"`
			if tt.confirm != "" {
				body += `,"confirm_password":"` + tt.confirm + `"`
			}
			code, resp := postJSON(t, resetServer(), resetPasswordPath, body+`}`)
			if code != tt.status {
				t.Fatalf("status = %d, want %d (body: %+v)", code, tt.status, resp)
			}
			if got := resp.Details["confirm_password"]; got != tt.detail {
				t.Errorf("details[confirm_password] = %q, want %q", got, tt.detail)
			}
		})
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

// Each limited route tells the client how long to wait, at most its own refill
// time: 12s for login (5 per minute), 60s and 600s for the token routes.
func TestLimitedRoutes_429CarriesRetryAfter(t *testing.T) {
	tests := []struct {
		name   string
		method string
		path   string
		maxSec int
	}{
		{"POST login", http.MethodPost, loginPath, 12},
		{"GET verify-email", http.MethodGet, verifyEmailPath, 60},
		{"POST forgot-password", http.MethodPost, forgotPasswordPath, 600},
		{"POST reset-password", http.MethodPost, resetPasswordPath, 60},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			e := newTestServer()
			var rec *httptest.ResponseRecorder
			for i := 0; i < 4; i++ {
				rec = httptest.NewRecorder()
				e.ServeHTTP(rec, httptest.NewRequestWithContext(context.Background(), tt.method, tt.path, nil))
			}

			if rec.Code != http.StatusTooManyRequests {
				t.Fatalf("request 4: status = %d, want %d", rec.Code, http.StatusTooManyRequests)
			}
			secs, err := strconv.Atoi(rec.Header().Get("Retry-After"))
			if err != nil {
				t.Fatalf("Retry-After = %q, want an integer: %v", rec.Header().Get("Retry-After"), err)
			}
			if secs < 1 || secs > tt.maxSec {
				t.Errorf("Retry-After = %d, want 1..%d", secs, tt.maxSec)
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
