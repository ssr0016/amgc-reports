package router

import (
	"context"
	"net/http"
	"strconv"
	"testing"

	"github.com/labstack/echo/v4"

	"github.com/ssr0016/pastoral-reports/backend/internal/accesscontrol"
	"github.com/ssr0016/pastoral-reports/backend/internal/apperror"
	authrest "github.com/ssr0016/pastoral-reports/backend/internal/auth/rest"
	reportrest "github.com/ssr0016/pastoral-reports/backend/internal/report/rest"
	"github.com/ssr0016/pastoral-reports/backend/internal/request"
	"github.com/ssr0016/pastoral-reports/backend/internal/user"
	userrest "github.com/ssr0016/pastoral-reports/backend/internal/user/rest"
	"github.com/ssr0016/pastoral-reports/backend/internal/user/userimpl"
	"github.com/ssr0016/pastoral-reports/backend/internal/validator"
)

// okUserSvc answers every call successfully and counts them: a call proves a
// request got past the gates.
type okUserSvc struct{ calls int }

func (s *okUserSvc) Create(context.Context, int64, string, user.CreateUserRequest) (*user.User, error) {
	s.calls++
	return &user.User{ID: 5, Email: "pia@example.com", Name: "Pia"}, nil
}

func (s *okUserSvc) Update(context.Context, int64, string, int64, user.UpdateUserRequest) (*user.User, error) {
	s.calls++
	return &user.User{ID: 1, Email: "pia@example.com", Name: "Pia"}, nil
}

// adminUsersServer wires the real routes. A request is user 1 when it carries
// X-Test-User; that user has the named role, and the role holds exactly perms.
func (s *okUserSvc) ChangeStatus(context.Context, int64, string, int64, string) (*user.User, error) {
	s.calls++
	return &user.User{ID: 1, Email: "pia@example.com", Name: "Pia", Status: user.UserStatusInactive}, nil
}

func (s *okUserSvc) ResetPassword(context.Context, int64, string, int64, string) error {
	s.calls++
	return nil
}

func (s *okUserSvc) Delete(context.Context, int64, string, int64, string) (*user.User, error) {
	s.calls++
	return &user.User{ID: 1, Email: "pia@example.com", Name: "Pia"}, nil
}

func (s *okUserSvc) Restore(context.Context, int64, string, int64) (*user.User, error) {
	s.calls++
	return &user.User{ID: 1, Email: "pia@example.com", Name: "Pia"}, nil
}

func (s *okUserSvc) ChangeRole(context.Context, int64, string, int64, int64) (*user.User, error) {
	s.calls++
	return &user.User{ID: 1, Email: "pia@example.com", Name: "Pia"}, nil
}

func adminUsersServer(roleName string, perms ...string) (*echo.Echo, *okUserSvc) {
	return adminUsersServerWith(Limiters{Global: noLimit, Read: noLimit, Write: noLimit}, roleName, perms...)
}

func adminUsersServerWith(limits Limiters, roleName string, perms ...string) (*echo.Echo, *okUserSvc) {
	users := userimpl.NewMockStore()
	users.GetWithRoleFunc = func(_ context.Context, id int64) (*user.User, error) {
		return &user.User{ID: id, RoleID: 9, Role: &accesscontrol.Role{Name: roleName}}, nil
	}
	users.GetByIDFunc = func(_ context.Context, id int64) (*user.User, error) {
		return &user.User{ID: id, Email: "pia@example.com", Name: "Pia"}, nil
	}
	svc := &okUserSvc{}

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
		authrest.NewAuthHandler(nil, nil), userrest.NewUserHandler(users), nil, nil,
		userrest.NewAdminUserHandler(users, svc),
		authrest.NewVerificationHandler(nil), authrest.NewPasswordResetHandler(nil),
		reportrest.NewReportHandler(okReportSvc{}), nil,
		nil, limits,
	)
	return e, svc
}

var adminUserRoutes = []struct {
	name, method, path, body string
	ok                       int
}{
	{"create", http.MethodPost, "/api/v1/admin/users",
		`{"email":"pia@example.com","name":"Pia","password":"X7kP9mQ2vL8nR4tY6wB3zC5dF1gH0jK","confirm_password":"X7kP9mQ2vL8nR4tY6wB3zC5dF1gH0jK"}`, http.StatusCreated},
	{"update", http.MethodPut, "/api/v1/admin/users/1", `{"name":"Pia"}`, http.StatusOK},
	{"status", http.MethodPut, "/api/v1/admin/users/1/status", `{"status":"inactive"}`, http.StatusOK},
	{"delete", http.MethodDelete, "/api/v1/admin/users/1", `{"reason":"left"}`, http.StatusOK},
	{"restore", http.MethodPost, "/api/v1/admin/users/1/restore", "", http.StatusOK},
	{"role", http.MethodPut, "/api/v1/admin/users/1/role", `{"role_id":4}`, http.StatusOK},
	{"reset-password", http.MethodPost, "/api/v1/admin/users/1/reset-password",
		`{"new_password":"X7kP9mQ2vL8nR4tY6wB3zC5dF1gH0jK","confirm_password":"X7kP9mQ2vL8nR4tY6wB3zC5dF1gH0jK"}`, http.StatusNoContent},
}

func TestAdminUserRoutes_Access(t *testing.T) {
	tests := []struct {
		name  string
		role  string
		perms []string
		user  string // "" = no session
		want  func(ok int) int
	}{
		{"no session", "admin", []string{"users:write"}, "", func(int) int { return http.StatusUnauthorized }},
		{"plain user", "user", []string{"users:read"}, "1", func(int) int { return http.StatusForbidden }},
		{"editor holding users:write is still not an admin", "editor", []string{"users:read", "users:write"}, "1", func(int) int { return http.StatusForbidden }},
		// users:write includes password reset: a custom role must never manage accounts (37b).
		{"custom role holding users:write and every admin-area permission", "Reviewer", reviewerPerms, "1", func(int) int { return http.StatusForbidden }},
		{"admin role without users:write", "admin", []string{"users:read"}, "1", func(int) int { return http.StatusForbidden }},
		{"admin with users:write", "admin", []string{"users:read", "users:write"}, "1", func(ok int) int { return ok }},
	}
	for _, tt := range tests {
		for _, rt := range adminUserRoutes {
			t.Run(tt.name+"/"+rt.name, func(t *testing.T) {
				e, svc := adminUsersServer(tt.role, tt.perms...)
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

// GET /users and /users/:id expose every account's email and name, so they need
// users:list (37b). The plain user and the editor hold users:read and still get 403.
func TestUsersRoutes_NeedUsersList(t *testing.T) {
	tests := []struct {
		name, role, user string // user "" = no session
		perms            []string
		want             int
	}{
		{"no session", "admin", "", []string{"users:list"}, http.StatusUnauthorized},
		{"plain user", "user", "1", plainUserPerms, http.StatusForbidden},
		{"editor holding users:read", "editor", "1", []string{"users:read", "users:write"}, http.StatusForbidden},
		{"admin role without users:list", "admin", "1", []string{"users:read"}, http.StatusForbidden},
		{"custom role with users:list", "Reviewer", "1", reviewerPerms, http.StatusOK},
		{"admin", "admin", "1", []string{"users:list"}, http.StatusOK},
	}
	for _, tt := range tests {
		for _, path := range []string{"/api/v1/users", "/api/v1/users/1"} {
			t.Run(tt.name+path, func(t *testing.T) {
				e, _ := adminUsersServer(tt.role, tt.perms...)
				if got := reportRequestAs(e, http.MethodGet, path, "", tt.user); got != tt.want {
					t.Errorf("status = %d, want %d", got, tt.want)
				}
			})
		}
	}
}

// The Edit User page's read stays admin-only, even for a users:list holder.
func TestAdminUserGet_AdminOnly(t *testing.T) {
	e, _ := adminUsersServer("Reviewer", reviewerPerms...)
	if got := reportRequestAs(e, http.MethodGet, "/api/v1/admin/users/1", "", "1"); got != http.StatusForbidden {
		t.Errorf("status = %d, want 403", got)
	}
}

// Self-registration is gone: accounts come only from POST /admin/users. The
// path may answer 401 (the protected /auth group catches unmatched paths before
// a 404) or 404/405, but never create an account or reach a service.
func TestAuthRegister_IsRemoved(t *testing.T) {
	const body = `{"email":"x@example.com","name":"Xavier","password":"X7kP9mQ2vL8nR4tY6wB3zC5dF1gH0jK"}`
	for _, user := range []string{"", "1"} {
		e, svc := adminUsersServer("admin", "users:write")
		switch got := reportRequestAs(e, http.MethodPost, "/api/v1/auth/register", body, user); got {
		case http.StatusUnauthorized, http.StatusNotFound, http.StatusMethodNotAllowed:
		default:
			t.Errorf("user %q: POST /auth/register = %d, want 401, 404 or 405", user, got)
		}
		if svc.calls != 0 {
			t.Errorf("user %q: register must not reach any service", user)
		}
	}
}

// Setting a password is a write: the route sits behind the same write limiter as
// change-password, after the role and permission gates (so the limiter only sees admins).
func TestResetPasswordRoute_IsRateLimited(t *testing.T) {
	tooMany := func(echo.HandlerFunc) echo.HandlerFunc {
		return func(echo.Context) error { return apperror.RateLimit("slow down") }
	}
	const body = `{"new_password":"X7kP9mQ2vL8nR4tY6wB3zC5dF1gH0jK","confirm_password":"X7kP9mQ2vL8nR4tY6wB3zC5dF1gH0jK"}`
	e, svc := adminUsersServerWith(Limiters{Global: noLimit, Read: noLimit, Write: tooMany}, "admin", "users:write")

	if got := reportRequestAs(e, http.MethodPost, "/api/v1/admin/users/1/reset-password", body, "1"); got != http.StatusTooManyRequests {
		t.Fatalf("status = %d, want 429 from the write limiter", got)
	}
	if svc.calls != 0 {
		t.Error("a limited request reached the service")
	}
}
