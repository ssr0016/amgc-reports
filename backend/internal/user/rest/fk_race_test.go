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
	acrest "github.com/ssr0016/pastoral-reports/backend/internal/accesscontrol/rest"
	"github.com/ssr0016/pastoral-reports/backend/internal/apperror"
	"github.com/ssr0016/pastoral-reports/backend/internal/database"
	"github.com/ssr0016/pastoral-reports/backend/internal/user"
	"github.com/ssr0016/pastoral-reports/backend/internal/user/userimpl"
	"github.com/ssr0016/pastoral-reports/backend/internal/validator"
)

// These tests cover the window between the handler's exists-check and the
// write: the row passes the check, is deleted, and the write then trips a
// foreign key. That must be a 404, not a 500.

type assignRoleRepo struct {
	accesscontrol.RoleStore
	assignErr error
}

func (f *assignRoleRepo) GetByID(_ context.Context, id int64) (*accesscontrol.Role, error) {
	return &accesscontrol.Role{ID: id}, nil
}

func (f *assignRoleRepo) AssignPermission(context.Context, int64, int64) error { return f.assignErr }

func (f *assignRoleRepo) GetPermissions(context.Context, int64) ([]accesscontrol.Permission, error) {
	return nil, nil
}

type existingPermissionRepo struct {
	accesscontrol.PermissionStore
}

func (existingPermissionRepo) GetByID(_ context.Context, id int64) (*accesscontrol.Permission, error) {
	return &accesscontrol.Permission{ID: id}, nil
}

func servePost(t *testing.T, route string, h echo.HandlerFunc, target, body string) (int, errorBody) {
	t.Helper()
	e := echo.New()
	e.HTTPErrorHandler = apperror.ErrorHandler
	e.Validator = validator.New()
	e.POST(route, h)

	req := httptest.NewRequestWithContext(context.Background(), http.MethodPost, target, strings.NewReader(body))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	var out errorBody
	if rec.Body.Len() > 0 {
		_ = json.Unmarshal(rec.Body.Bytes(), &out)
	}
	return rec.Code, out
}

func TestRoleHandler_AssignPermission_WriteErrors(t *testing.T) {
	tests := []struct {
		name        string
		assignErr   error
		wantStatus  int
		wantCode    string
		wantMessage string
	}{
		{"assigned", nil, http.StatusOK, "", "permission assigned"},
		{"role or permission vanished mid-assign is 404",
			fmt.Errorf("assign permission: %w", database.ErrForeignKeyViolation),
			http.StatusNotFound, "NOT_FOUND", "role or permission not found"},
		{"database failure stays 500", errors.New("connection refused"),
			http.StatusInternalServerError, "INTERNAL_ERROR", "failed to assign permission"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			h := acrest.NewRoleHandler(&assignRoleRepo{assignErr: tt.assignErr}, existingPermissionRepo{})

			status, body := servePost(t, "/admin/roles/:id/permissions", h.AssignPermission,
				"/admin/roles/7/permissions", `{"permission_id": 3}`)

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

type existingRoleRepo struct{ accesscontrol.RoleStore }

func (existingRoleRepo) GetByID(_ context.Context, id int64) (*accesscontrol.Role, error) {
	return &accesscontrol.Role{ID: id}, nil
}

func TestAdminUserHandler_ChangeRole_WriteErrors(t *testing.T) {
	tests := []struct {
		name        string
		updateErr   error
		wantStatus  int
		wantCode    string
		wantMessage string
	}{
		{"role vanished mid-update is 404",
			fmt.Errorf("update role: %w", database.ErrForeignKeyViolation),
			http.StatusNotFound, "NOT_FOUND", "role not found"},
		{"user vanished mid-update is 404 with its own message",
			fmt.Errorf("users: %w", database.ErrNotFound),
			http.StatusNotFound, "NOT_FOUND", "user not found"},
		{"database failure stays 500", errors.New("connection refused"),
			http.StatusInternalServerError, "INTERNAL_ERROR", "failed to update role"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			users := userimpl.NewMockStore()
			users.GetByIDFunc = func(_ context.Context, id int64) (*user.User, error) {
				return &user.User{ID: id}, nil
			}
			users.UpdateRoleFunc = func(context.Context, int64, int64) error { return tt.updateErr }
			h := NewAdminUserHandler(users, existingRoleRepo{}, nil)

			status, body := servePost(t, "/admin/users/:id/role", h.ChangeRole,
				"/admin/users/5/role", `{"role_id": 9}`)

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

// errorBody is the JSON error body apperror writes.
type errorBody struct {
	Error   string `json:"error"`
	Message string `json:"message"`
}
