package handler

import (
	"context"
	"fmt"
	"net/http"
	"testing"

	"github.com/ssr0016/pastoral-reports/backend/internal/model"
	"github.com/ssr0016/pastoral-reports/backend/internal/repository"
)

type protectedRoleRepo struct {
	repository.RoleRepository
	err error
}

func (f *protectedRoleRepo) Delete(context.Context, int64) error { return f.err }

func (f *protectedRoleRepo) Update(context.Context, int64, model.UpdateRoleRequest) (*model.Role, error) {
	return nil, f.err
}

func TestRoleHandler_DefaultRoleIsProtected(t *testing.T) {
	protected := fmt.Errorf("roles: %w", repository.ErrProtectedRole)
	h := NewRoleHandler(&protectedRoleRepo{err: protected}, nil)

	t.Run("delete is 409", func(t *testing.T) {
		status, body := serveDelete(t, "/admin/roles/:id", h.Delete, "/admin/roles/3")

		if status != http.StatusConflict || body.Error != "CONFLICT" || body.Message != "default role cannot be deleted" {
			t.Errorf("got %d %q %q", status, body.Error, body.Message)
		}
	})

	t.Run("rename is 409", func(t *testing.T) {
		status, body := serveJSON(t, http.MethodPut, "/admin/roles/:id", h.Update, "/admin/roles/3", `{"name":"member"}`)

		if status != http.StatusConflict || body.Error != "CONFLICT" || body.Message != "default role cannot be renamed" {
			t.Errorf("got %d %q %q", status, body.Error, body.Message)
		}
	})
}
