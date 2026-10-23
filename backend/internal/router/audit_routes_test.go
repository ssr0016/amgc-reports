package router

import (
	"context"
	"net/http"
	"strconv"
	"testing"

	"github.com/labstack/echo/v4"

	"github.com/ssr0016/pastoral-reports/backend/internal/accesscontrol"
	"github.com/ssr0016/pastoral-reports/backend/internal/apperror"
	"github.com/ssr0016/pastoral-reports/backend/internal/audit"
	auditrest "github.com/ssr0016/pastoral-reports/backend/internal/audit/rest"
	authrest "github.com/ssr0016/pastoral-reports/backend/internal/auth/rest"
	"github.com/ssr0016/pastoral-reports/backend/internal/request"
	"github.com/ssr0016/pastoral-reports/backend/internal/user"
	"github.com/ssr0016/pastoral-reports/backend/internal/user/userimpl"
	"github.com/ssr0016/pastoral-reports/backend/pkg/pagination"
)

// okAuditSvc answers every call and counts them: a call proves a request got past the gates.
type okAuditSvc struct{ calls int }

func (s *okAuditSvc) List(context.Context, audit.ListQuery, pagination.Params) ([]audit.Log, int64, error) {
	s.calls++
	return nil, 0, nil
}

func auditServer(roleName string, perms ...string) (*echo.Echo, *okAuditSvc) {
	users := userimpl.NewMockStore()
	users.GetWithRoleFunc = func(_ context.Context, id int64) (*user.User, error) {
		return &user.User{ID: id, RoleID: 9, Role: &accesscontrol.Role{Name: roleName}}, nil
	}
	svc := &okAuditSvc{}
	e := echo.New()
	e.HTTPErrorHandler = apperror.ErrorHandler
	e.Use(func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			if id, err := strconv.ParseInt(c.Request().Header.Get("X-Test-User"), 10, 64); err == nil {
				c.SetRequest(c.Request().WithContext(request.WithUser(c.Request().Context(), request.User{ID: id})))
			}
			return next(c)
		}
	})
	users.Roles = permRoles{perms: perms}
	Setup(e, noAuthn, users,
		authrest.NewAuthHandler(nil, nil), nil, nil, nil, nil,
		authrest.NewVerificationHandler(nil), authrest.NewPasswordResetHandler(nil),
		nil, nil, auditrest.NewAuditHandler(svc),
		Limiters{Global: noLimit, Read: noLimit, Write: noLimit},
	)
	return e, svc
}

// The audit trail needs audit:read (37b). An editor, who holds users:write, still gets 403.
func TestAuditRoute_NeedsAuditRead(t *testing.T) {
	tests := []struct {
		name, role, user string // user "" = no session
		perms            []string
		want             int
		reaches          bool
	}{
		{"no session", "admin", "", []string{"audit:read"}, http.StatusUnauthorized, false},
		{"plain user", "user", "1", plainUserPerms, http.StatusForbidden, false},
		{"editor", "editor", "1", []string{"users:read", "users:write"}, http.StatusForbidden, false},
		{"admin role without audit:read", "admin", "1", nil, http.StatusForbidden, false},
		{"custom role with audit:read", "Reviewer", "1", reviewerPerms, http.StatusOK, true},
		{"admin", "admin", "1", []string{"audit:read"}, http.StatusOK, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			e, svc := auditServer(tt.role, tt.perms...)
			if got := reportRequestAs(e, http.MethodGet, "/api/v1/admin/audit-logs?month=May&year=2026", "", tt.user); got != tt.want {
				t.Fatalf("status = %d, want %d", got, tt.want)
			}
			if (svc.calls == 1) != tt.reaches {
				t.Errorf("service calls = %d: only an audit:read holder may reach it", svc.calls)
			}
		})
	}
}

// The trail is read-only: no route writes, changes or deletes a row.
func TestAuditRoute_ReadOnly(t *testing.T) {
	e, _ := auditServer("admin", "audit:read")
	for _, m := range []string{http.MethodPost, http.MethodPut, http.MethodPatch, http.MethodDelete} {
		if got := reportRequestAs(e, m, "/api/v1/admin/audit-logs", "", "1"); got != http.StatusMethodNotAllowed && got != http.StatusNotFound {
			t.Errorf("%s /admin/audit-logs = %d, want 404 or 405", m, got)
		}
	}
}
