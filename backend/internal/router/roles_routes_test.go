package router

import (
	"context"
	"net/http"
	"strconv"
	"testing"

	"github.com/labstack/echo/v4"

	"github.com/ssr0016/pastoral-reports/backend/internal/accesscontrol"
	acrest "github.com/ssr0016/pastoral-reports/backend/internal/accesscontrol/rest"
	"github.com/ssr0016/pastoral-reports/backend/internal/apperror"
	"github.com/ssr0016/pastoral-reports/backend/internal/request"
	"github.com/ssr0016/pastoral-reports/backend/internal/user"
	"github.com/ssr0016/pastoral-reports/backend/internal/user/userimpl"
	"github.com/ssr0016/pastoral-reports/backend/internal/validator"
)

// okRoleSvc answers every write successfully and counts the calls.
type okRoleSvc struct{ calls int }

func (s *okRoleSvc) Create(context.Context, int64, string, accesscontrol.CreateRoleRequest) (*accesscontrol.Role, error) {
	s.calls++
	return &accesscontrol.Role{ID: 4, Name: "Reviewer"}, nil
}

func (s *okRoleSvc) Update(context.Context, int64, string, int64, accesscontrol.UpdateRoleRequest) (*accesscontrol.Role, error) {
	s.calls++
	return &accesscontrol.Role{ID: 4, Name: "Reviewer"}, nil
}

func (s *okRoleSvc) Delete(context.Context, int64, string, int64) error {
	s.calls++
	return nil
}

// rolesServer wires the real routes. A request is user 1 when it carries
// X-Test-User; that user has the named role, and the role holds exactly perms.
func rolesServer(limits Limiters, roleName string, perms ...string) (*echo.Echo, *okRoleSvc) {
	users := userimpl.NewMockStore()
	users.GetWithRoleFunc = func(_ context.Context, id int64) (*user.User, error) {
		return &user.User{ID: id, RoleID: 9, Role: &accesscontrol.Role{Name: roleName}}, nil
	}
	svc := &okRoleSvc{}

	e := echo.New()
	e.Validator = validator.New()
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
		nil, nil, acrest.NewRoleHandler(nil, svc), nil, nil, nil, nil, nil, nil, nil, limits)
	return e, svc
}

var roleWriteRoutes = []struct {
	name, method, path, body string
	ok                       int
}{
	{"create", http.MethodPost, "/api/v1/admin/roles", `{"name":"Reviewer","permission_ids":[]}`, http.StatusCreated},
	{"update", http.MethodPut, "/api/v1/admin/roles/4", `{"name":"Reviewer","permission_ids":[]}`, http.StatusOK},
	{"delete", http.MethodDelete, "/api/v1/admin/roles/4", "", http.StatusNoContent},
}

func TestRoleWriteRoutes_Access(t *testing.T) {
	noLimits := Limiters{Global: noLimit, Read: noLimit, Write: noLimit}
	tests := []struct {
		name, role, user string // user "" = no session
		perms            []string
		want             func(ok int) int
	}{
		{"no session", "admin", "", []string{"roles:write"}, func(int) int { return http.StatusUnauthorized }},
		{"plain user", "user", "1", []string{"roles:write"}, func(int) int { return http.StatusForbidden }},
		{"custom role holding roles:write is still not an admin", "Reviewer", "1", []string{"roles:write"}, func(int) int { return http.StatusForbidden }},
		{"admin role without roles:write", "admin", "1", []string{"roles:read"}, func(int) int { return http.StatusForbidden }},
		{"admin with roles:write", "admin", "1", []string{"roles:write"}, func(ok int) int { return ok }},
	}
	for _, tt := range tests {
		for _, rt := range roleWriteRoutes {
			t.Run(tt.name+"/"+rt.name, func(t *testing.T) {
				e, svc := rolesServer(noLimits, tt.role, tt.perms...)
				got := reportRequestAs(e, rt.method, rt.path, rt.body, tt.user)
				if want := tt.want(rt.ok); got != want {
					t.Fatalf("status = %d, want %d", got, want)
				}
				if (got == rt.ok) != (svc.calls == 1) {
					t.Errorf("service calls = %d for status %d: only a request that passes every gate may reach it", svc.calls, got)
				}
			})
		}
	}
}

// The Roles tab and the permission list stay admin-only (37b): a custom role
// holding every admin-area permission and roles:write gets 403 on every route.
func TestRoleAndPermissionRoutes_AdminOnly(t *testing.T) {
	e, svc := rolesServer(Limiters{Global: noLimit, Read: noLimit, Write: noLimit}, "Reviewer", reviewerPerms...)
	for _, rt := range []struct{ method, path, body string }{
		{http.MethodGet, "/api/v1/admin/roles", ""},
		{http.MethodGet, "/api/v1/admin/roles/4", ""},
		{http.MethodGet, "/api/v1/admin/permissions", ""},
		{http.MethodGet, "/api/v1/admin/permissions/1", ""},
		{http.MethodPost, "/api/v1/admin/permissions", `{"name":"x:y","resource":"x","action":"y"}`},
		{http.MethodDelete, "/api/v1/admin/permissions/1", ""},
	} {
		if got := reportRequestAs(e, rt.method, rt.path, rt.body, "1"); got != http.StatusForbidden {
			t.Errorf("%s %s = %d, want 403", rt.method, rt.path, got)
		}
	}
	if svc.calls != 0 {
		t.Error("a custom role reached the role service")
	}
}

// Role writes share the write limiter, after the role and permission gates.
func TestRoleWriteRoutes_AreRateLimited(t *testing.T) {
	tooMany := func(echo.HandlerFunc) echo.HandlerFunc {
		return func(echo.Context) error { return apperror.RateLimit("slow down") }
	}
	for _, rt := range roleWriteRoutes {
		e, svc := rolesServer(Limiters{Global: noLimit, Read: noLimit, Write: tooMany}, "admin", "roles:write")
		if got := reportRequestAs(e, rt.method, rt.path, rt.body, "1"); got != http.StatusTooManyRequests {
			t.Errorf("%s: status = %d, want 429", rt.name, got)
		}
		if svc.calls != 0 {
			t.Errorf("%s: a limited request reached the service", rt.name)
		}
	}
}

// The unaudited per-permission routes are gone: PUT /admin/roles/:id replaces the set.
func TestRolePermissionRoutes_AreRemoved(t *testing.T) {
	e, svc := rolesServer(Limiters{Global: noLimit, Read: noLimit, Write: noLimit}, "admin", "roles:write")
	for _, rt := range []struct{ method, path, body string }{
		{http.MethodPost, "/api/v1/admin/roles/4/permissions", `{"permission_id":1}`},
		{http.MethodDelete, "/api/v1/admin/roles/4/permissions/1", ""},
	} {
		// Echo may hand the leftover path to DELETE /roles/:id, which refuses the id (400).
		switch got := reportRequestAs(e, rt.method, rt.path, rt.body, "1"); got {
		case http.StatusBadRequest, http.StatusNotFound, http.StatusMethodNotAllowed:
		default:
			t.Errorf("%s %s = %d, want 400, 404 or 405", rt.method, rt.path, got)
		}
	}
	if svc.calls != 0 {
		t.Error("a removed route reached the service")
	}
}
