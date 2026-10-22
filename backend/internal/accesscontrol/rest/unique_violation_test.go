package rest

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/labstack/echo/v4"

	"github.com/ssr0016/pastoral-reports/backend/internal/accesscontrol"
	"github.com/ssr0016/pastoral-reports/backend/internal/apperror"
	"github.com/ssr0016/pastoral-reports/backend/internal/database"
	"github.com/ssr0016/pastoral-reports/backend/internal/validator"
)

// The repo reports a unique violation as database.ErrUniqueViolation; the
// permission handler's name pre-check usually answers first, so the write error
// is only reached on a race. Role writes are covered in role_handler_test.go.

type uniquePermissionRepo struct {
	accesscontrol.PermissionStore
	writeErr error
}

func (f *uniquePermissionRepo) GetByName(context.Context, string) (*accesscontrol.Permission, error) {
	return nil, nil
}

func (f *uniquePermissionRepo) Create(context.Context, string, string, string) (*accesscontrol.Permission, error) {
	return &accesscontrol.Permission{ID: 1}, f.writeErr
}

func serveJSON(t *testing.T, method, route string, h echo.HandlerFunc, target, body string) (int, errorBody) {
	t.Helper()
	e := echo.New()
	e.HTTPErrorHandler = apperror.ErrorHandler
	e.Validator = validator.New()
	e.Add(method, route, h)

	req := httptest.NewRequestWithContext(context.Background(), method, target, strings.NewReader(body))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	var out errorBody
	if rec.Body.Len() > 0 {
		_ = json.Unmarshal(rec.Body.Bytes(), &out)
	}
	return rec.Code, out
}

func TestUniqueViolation_Handlers(t *testing.T) {
	unique := fmt.Errorf("scan role: %w: %w", database.ErrUniqueViolation, errors.New("SQLSTATE 23505"))

	tests := []struct {
		name        string
		run         func(writeErr error) (int, errorBody)
		writeErr    error
		wantStatus  int
		wantCode    string
		wantMessage string
	}{
		{"permission create losing the name race is 409", func(e error) (int, errorBody) {
			h := NewPermissionHandler(&uniquePermissionRepo{writeErr: e})
			return serveJSON(t, http.MethodPost, "/admin/permissions", h.Create, "/admin/permissions",
				`{"name":"report:read","resource":"report","action":"read"}`)
		}, unique, http.StatusConflict, "CONFLICT", "permission name already exists"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			status, body := tt.run(tt.writeErr)

			if status != tt.wantStatus {
				t.Errorf("status = %d, want %d", status, tt.wantStatus)
			}
			if body.Error != tt.wantCode {
				t.Errorf("error code = %q, want %q", body.Error, tt.wantCode)
			}
			if body.Message != tt.wantMessage {
				t.Errorf("message = %q, want %q", body.Message, tt.wantMessage)
			}
		})
	}
}
