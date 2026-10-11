package handler

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"testing"

	"github.com/jackc/pgx/v5/pgconn"

	"github.com/ssr0016/pastoral-reports/backend/internal/repository"
)

// fakePermissionRepo overrides only Delete (see fakeRoleRepo).
type fakePermissionRepo struct {
	repository.PermissionRepository
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
			fmt.Errorf("permissions: %w", repository.ErrNotFound),
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
