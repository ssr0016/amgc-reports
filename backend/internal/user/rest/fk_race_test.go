package rest

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"testing"

	"github.com/ssr0016/pastoral-reports/backend/internal/accesscontrol"
	"github.com/ssr0016/pastoral-reports/backend/internal/audit/auditimpl"
	"github.com/ssr0016/pastoral-reports/backend/internal/database"
	"github.com/ssr0016/pastoral-reports/backend/internal/user"
	"github.com/ssr0016/pastoral-reports/backend/internal/user/userimpl"
)

// These tests cover the window between the service's exists-checks and the
// write: the row passes the check, is deleted, and the write then trips a
// foreign key or matches no row. That must be a 404, not a 500.

type existingRoleRepo struct{ accesscontrol.RoleStore }

func (existingRoleRepo) GetByID(_ context.Context, id int64) (*accesscontrol.Role, error) {
	return &accesscontrol.Role{ID: id, Name: "Reviewer"}, nil
}

// directTx runs fn directly. The in-memory mocks have no transaction to roll back.
type directTx struct{}

func (directTx) InTransaction(ctx context.Context, fn func(context.Context) error) error {
	return fn(ctx)
}

func TestAdminUserHandler_ChangeRole_WriteErrors(t *testing.T) {
	tests := []struct {
		name        string
		updateErr   error
		wantStatus  int
		wantMessage string
	}{
		{"role vanished mid-update is 404",
			fmt.Errorf("update role: %w", database.ErrForeignKeyViolation),
			http.StatusNotFound, "role not found"},
		{"user vanished mid-update is 404 with its own message",
			fmt.Errorf("users: %w", database.ErrNotFound),
			http.StatusNotFound, "user not found"},
		{"database failure stays 500", errors.New("connection refused"),
			http.StatusInternalServerError, "failed to change role"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			users := userimpl.NewMockStore()
			users.GetWithRoleFunc = func(_ context.Context, id int64) (*user.User, error) {
				return &user.User{ID: id, RoleID: 3, Role: &accesscontrol.Role{Name: "user"}}, nil
			}
			users.UpdateRoleFunc = func(context.Context, int64, int64) error { return tt.updateErr }
			svc := userimpl.NewService(users, existingRoleRepo{}, auditimpl.NewMockStore(), directTx{})

			rec, body := serveUserAdmin(t, svc, 99, http.MethodPut, "/admin/users/5/role", `{"role_id": 9}`)

			if rec.Code != tt.wantStatus || body.Message != tt.wantMessage {
				t.Errorf("got %d %q, want %d %q", rec.Code, body.Message, tt.wantStatus, tt.wantMessage)
			}
		})
	}
}
