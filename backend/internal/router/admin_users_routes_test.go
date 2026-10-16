package router

import (
	"context"
	"net/http"
	"strconv"
	"testing"

	"github.com/alexedwards/scs/v2"
	"github.com/labstack/echo/v4"

	"github.com/ssr0016/pastoral-reports/backend/internal/apperror"
	"github.com/ssr0016/pastoral-reports/backend/internal/handler"
	"github.com/ssr0016/pastoral-reports/backend/internal/model"
	"github.com/ssr0016/pastoral-reports/backend/internal/repository"
	"github.com/ssr0016/pastoral-reports/backend/internal/validator"
)

// okUserSvc answers every call successfully and counts them: a call proves a
// request got past the gates.
type okUserSvc struct{ calls int }

func (s *okUserSvc) Create(context.Context, int64, string, model.CreateUserRequest) (*model.User, error) {
	s.calls++
	return &model.User{ID: 5, Email: "pia@example.com", Name: "Pia"}, nil
}

func (s *okUserSvc) Update(context.Context, int64, string, int64, model.UpdateUserRequest) (*model.User, error) {
	s.calls++
	return &model.User{ID: 1, Email: "pia@example.com", Name: "Pia"}, nil
}

// adminUsersServer wires the real routes. A request is user 1 when it carries
// X-Test-User; that user has the named role, and the role holds exactly perms.
func adminUsersServer(roleName string, perms ...string) (*echo.Echo, *okUserSvc) {
	sm := scs.New()
	users := repository.NewMockUserRepo()
	users.GetWithRoleFunc = func(_ context.Context, id int64) (*model.User, error) {
		return &model.User{ID: id, RoleID: 9, Role: &model.Role{Name: roleName}}, nil
	}
	users.GetByIDFunc = func(_ context.Context, id int64) (*model.User, error) {
		return &model.User{ID: id, Email: "pia@example.com", Name: "Pia"}, nil
	}
	svc := &okUserSvc{}

	e := echo.New()
	e.Validator = validator.New()
	e.HTTPErrorHandler = apperror.ErrorHandler
	e.Use(echo.WrapMiddleware(sm.LoadAndSave))
	e.Use(func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			if id, err := strconv.ParseInt(c.Request().Header.Get("X-Test-User"), 10, 64); err == nil {
				sm.Put(c.Request().Context(), "user_id", id)
			}
			return next(c)
		}
	})
	Setup(e, sm, users, permRoles{perms: perms},
		handler.NewAuthHandler(nil, nil), handler.NewUserHandler(users), nil, nil,
		handler.NewAdminUserHandler(users, nil, svc, sm),
		handler.NewVerificationHandler(nil), handler.NewPasswordResetHandler(nil),
		handler.NewReportHandler(okReportSvc{}, sm), nil,
		Limiters{Global: noLimit, Read: noLimit, Write: noLimit},
	)
	return e, svc
}

var adminUserRoutes = []struct {
	name, method, path, body string
	ok                       int
}{
	{"create", http.MethodPost, "/api/v1/admin/users",
		`{"email":"pia@example.com","name":"Pia","password":"X7kP9mQ2vL8nR4tY6wB3zC5dF1gH0jK"}`, http.StatusCreated},
	{"update", http.MethodPut, "/api/v1/admin/users/1", `{"name":"Pia"}`, http.StatusOK},
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
// the admin role. The editor holds users:read and still gets 403: the role gates
// the routes, not the permission.
func TestUsersRoutes_AdminOnly(t *testing.T) {
	tests := []struct {
		name, role, user string // user "" = no session
		want             int
	}{
		{"no session", "admin", "", http.StatusUnauthorized},
		{"plain user", "user", "1", http.StatusForbidden},
		{"editor holding users:read", "editor", "1", http.StatusForbidden},
		{"admin", "admin", "1", http.StatusOK},
	}
	for _, tt := range tests {
		for _, path := range []string{"/api/v1/users", "/api/v1/users/1"} {
			t.Run(tt.name+path, func(t *testing.T) {
				e, _ := adminUsersServer(tt.role, "users:read")
				if got := reportRequestAs(e, http.MethodGet, path, "", tt.user); got != tt.want {
					t.Errorf("status = %d, want %d", got, tt.want)
				}
			})
		}
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
