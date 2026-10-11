package handler

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/labstack/echo/v4"

	"github.com/ssr0016/pastoral-reports/backend/internal/apperror"
	"github.com/ssr0016/pastoral-reports/backend/internal/repository"
)

// fakeRoleRepo overrides only Delete; any other method panics on the nil
// embedded interface, which keeps these tests honest about what they touch.
type fakeRoleRepo struct {
	repository.RoleRepository
	deleteErr error
}

func (f *fakeRoleRepo) Delete(context.Context, int64) error { return f.deleteErr }

type errorBody struct {
	Error   string `json:"error"`
	Message string `json:"message"`
}

func serveDelete(t *testing.T, route string, h echo.HandlerFunc, target string) (int, errorBody) {
	t.Helper()
	e := echo.New()
	e.HTTPErrorHandler = apperror.ErrorHandler
	e.DELETE(route, h)

	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, httptest.NewRequestWithContext(context.Background(), http.MethodDelete, target, nil))

	var body errorBody
	if rec.Body.Len() > 0 {
		if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
			t.Fatalf("decode body %q: %v", rec.Body.String(), err)
		}
	}
	return rec.Code, body
}

func TestRoleHandler_Delete(t *testing.T) {
	tests := []struct {
		name        string
		target      string
		repoErr     error
		wantStatus  int
		wantCode    string
		wantMessage string
	}{
		{"deleted", "/admin/roles/7", nil, http.StatusNoContent, "", ""},
		{"missing role is 404", "/admin/roles/7",
			fmt.Errorf("roles: %w", repository.ErrNotFound),
			http.StatusNotFound, "NOT_FOUND", "role not found"},
		{"role still assigned to users is 409", "/admin/roles/7",
			fmt.Errorf("delete roles: %w", repository.ErrForeignKeyViolation),
			http.StatusConflict, "CONFLICT", "role is assigned to users"},
		{"database failure stays 500", "/admin/roles/7",
			errors.New("connection refused"),
			http.StatusInternalServerError, "INTERNAL_ERROR", "failed to delete role"},
		{"invalid id is 400", "/admin/roles/abc", nil,
			http.StatusBadRequest, "BAD_REQUEST", "invalid role id"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			h := NewRoleHandler(&fakeRoleRepo{deleteErr: tt.repoErr}, nil)

			status, body := serveDelete(t, "/admin/roles/:id", h.Delete, tt.target)

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
