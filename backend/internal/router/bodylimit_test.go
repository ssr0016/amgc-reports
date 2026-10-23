package router

import (
	"bytes"
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/labstack/echo/v4"
	echomw "github.com/labstack/echo/v4/middleware"

	"github.com/ssr0016/pastoral-reports/backend/internal/apperror"
	authrest "github.com/ssr0016/pastoral-reports/backend/internal/auth/rest"
	ourmiddleware "github.com/ssr0016/pastoral-reports/backend/internal/middleware"
	"github.com/ssr0016/pastoral-reports/backend/internal/validator"
)

// The limit is global, so it must reach every route that binds a body,
// including ones behind auth: the check happens before RequireAuth runs.
func TestBodyLimit_AppliesToAllBindRoutes(t *testing.T) {
	e := echo.New()
	e.Validator = validator.New()
	e.HTTPErrorHandler = apperror.ErrorHandler
	e.Use(ourmiddleware.BodyLimit())
	e.Use(echomw.Recover())
	Setup(e, noAuthn, nil,
		authrest.NewAuthHandler(nil, nil),
		nil, nil, nil, nil,
		authrest.NewVerificationHandler(nil),
		authrest.NewPasswordResetHandler(nil),
		nil, nil, nil, noLimits,
	)

	routes := []struct {
		name string
		path string
	}{
		{"login", loginPath},
		{"register", "/api/v1/auth/register"},
		{"forgot password", forgotPasswordPath},
		{"reset password", resetPasswordPath},
		{"admin create role", "/api/v1/admin/roles"},
	}

	body := bytes.Repeat([]byte("a"), ourmiddleware.MaxRequestBodyBytes+1)
	for _, tt := range routes {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequestWithContext(context.Background(), http.MethodPost, tt.path, bytes.NewReader(body))
			rec := httptest.NewRecorder()

			e.ServeHTTP(rec, req)

			if rec.Code != http.StatusRequestEntityTooLarge {
				t.Errorf("POST %s status = %d, want %d", tt.path, rec.Code, http.StatusRequestEntityTooLarge)
			}
		})
	}
}
