package rest

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/jackc/pgx/v5/pgconn"
	"github.com/labstack/echo/v4"

	"github.com/ssr0016/pastoral-reports/backend/internal/accesscontrol"
	"github.com/ssr0016/pastoral-reports/backend/internal/apperror"
	"github.com/ssr0016/pastoral-reports/backend/internal/database"
)

// fakePermissionRepo overrides only Delete; any other method panics on the nil
// embedded interface, which keeps these tests honest about what they touch.
type fakePermissionRepo struct {
	accesscontrol.PermissionStore
	deleteErr error
}

func (f *fakePermissionRepo) Delete(context.Context, int64) error { return f.deleteErr }

func TestPermissionHandler_Delete(t *testing.T) {
	tests := []struct {
		name        string
		target      string
		repoErr     error
		wantStatus  int
		wantCode    string
		wantMessage string
	}{
		{"deleted", "/admin/permissions/7", nil, http.StatusNoContent, "", ""},
		{"missing permission is 404", "/admin/permissions/7",
			fmt.Errorf("permissions: %w", database.ErrNotFound),
			http.StatusNotFound, "NOT_FOUND", "permission not found"},
		{"foreign key violation stays 500", "/admin/permissions/7",
			&pgconn.PgError{Code: "23503", Message: "violates foreign key constraint"},
			http.StatusInternalServerError, "INTERNAL_ERROR", "failed to delete permission"},
		{"database failure stays 500", "/admin/permissions/7",
			errors.New("connection refused"),
			http.StatusInternalServerError, "INTERNAL_ERROR", "failed to delete permission"},
		{"invalid id is 400", "/admin/permissions/abc", nil,
			http.StatusBadRequest, "BAD_REQUEST", "invalid permission id"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			h := NewPermissionHandler(&fakePermissionRepo{deleteErr: tt.repoErr})

			status, body := serveDelete(t, "/admin/permissions/:id", h.Delete, tt.target)

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
