package rest

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/labstack/echo/v4"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ssr0016/pastoral-reports/backend/internal/apperror"
	"github.com/ssr0016/pastoral-reports/backend/internal/auth"
	"github.com/ssr0016/pastoral-reports/backend/internal/request"
	"github.com/ssr0016/pastoral-reports/backend/internal/user"
	"github.com/ssr0016/pastoral-reports/backend/internal/validator"
)

// fakeUserSvc returns err from every method and records what it was called with.
type fakeUserSvc struct {
	err         error
	calls       int
	gotActor    int64
	gotIP       string
	gotID       int64
	gotCreate   user.CreateUserRequest
	gotUpdate   user.UpdateUserRequest
	gotStatus   string
	gotPassword string
	gotReason   string
	gotRoleID   int64
}

func (f *fakeUserSvc) ChangeRole(_ context.Context, actor int64, ip string, id, roleID int64) (*user.User, error) {
	f.calls++
	f.gotActor, f.gotIP, f.gotID, f.gotRoleID = actor, ip, id, roleID
	return &user.User{ID: id, Email: "u@example.com", Name: "U", RoleID: roleID, PasswordHash: "secret-hash"}, f.err
}

func (f *fakeUserSvc) Create(_ context.Context, actor int64, ip string, req user.CreateUserRequest) (*user.User, error) {
	f.calls++
	f.gotActor, f.gotIP, f.gotCreate = actor, ip, req
	a := req.AreaAssignment
	return &user.User{ID: 5, Email: req.Email, Name: req.Name, PasswordHash: "secret-hash", AreaAssignment: &a}, f.err
}

func (f *fakeUserSvc) Update(_ context.Context, actor int64, ip string, id int64, req user.UpdateUserRequest) (*user.User, error) {
	f.calls++
	f.gotActor, f.gotIP, f.gotID, f.gotUpdate = actor, ip, id, req
	return &user.User{ID: id, Email: "u@example.com", Name: req.Name, PasswordHash: "secret-hash"}, f.err
}

func (f *fakeUserSvc) ChangeStatus(_ context.Context, actor int64, ip string, id int64, status string) (*user.User, error) {
	f.calls++
	f.gotActor, f.gotIP, f.gotID, f.gotStatus = actor, ip, id, status
	return &user.User{ID: id, Email: "u@example.com", Name: "U", Status: status, PasswordHash: "secret-hash"}, f.err
}

func (f *fakeUserSvc) ResetPassword(_ context.Context, actor int64, ip string, id int64, password string) error {
	f.calls++
	f.gotActor, f.gotIP, f.gotID, f.gotPassword = actor, ip, id, password
	return f.err
}

func (f *fakeUserSvc) Delete(_ context.Context, actor int64, ip string, id int64, reason string) (*user.User, error) {
	f.calls++
	f.gotActor, f.gotIP, f.gotID, f.gotReason = actor, ip, id, reason
	now := time.Now()
	return &user.User{ID: id, Email: "u@example.com", Name: "U", DeletedAt: &now, PasswordHash: "secret-hash"}, f.err
}

func (f *fakeUserSvc) Restore(_ context.Context, actor int64, ip string, id int64) (*user.User, error) {
	f.calls++
	f.gotActor, f.gotIP, f.gotID = actor, ip, id
	return &user.User{ID: id, Email: "u@example.com", Name: "U", PasswordHash: "secret-hash"}, f.err
}

type userErrBody struct {
	Error   string            `json:"error"`
	Message string            `json:"message"`
	Details map[string]string `json:"details"`
}

func serveUserAdmin(t *testing.T, svc user.Service, sessionUser int64, method, target, body string) (*httptest.ResponseRecorder, userErrBody) {
	t.Helper()
	h := NewAdminUserHandler(nil, svc)

	e := echo.New()
	e.Validator = validator.New()
	e.HTTPErrorHandler = apperror.ErrorHandler
	e.Use(func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			if sessionUser != 0 {
				c.SetRequest(c.Request().WithContext(request.WithUser(c.Request().Context(), request.User{ID: sessionUser})))
			}
			return next(c)
		}
	})
	e.POST("/admin/users", h.CreateUser)
	e.PUT("/admin/users/:id", h.UpdateUser)
	e.PUT("/admin/users/:id/status", h.ChangeStatus)
	e.POST("/admin/users/:id/reset-password", h.ResetPassword)
	e.DELETE("/admin/users/:id", h.DeleteUser)
	e.POST("/admin/users/:id/restore", h.RestoreUser)
	e.PUT("/admin/users/:id/role", h.ChangeRole)

	req := httptest.NewRequestWithContext(context.Background(), method, target, strings.NewReader(body))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	req.RemoteAddr = "10.1.2.3:5555"
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	var eb userErrBody
	if rec.Code >= 400 {
		require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &eb), rec.Body.String())
	}
	return rec, eb
}

const strongPassword = "X7kP9mQ2vL8nR4tY6wB3zC5dF1gH0jK"

func createBody(extra string) string { return createBodyConfirm(strongPassword, extra) }

// createBodyConfirm is createBody with an explicit confirmation ("" leaves the field out).
func createBodyConfirm(confirm, extra string) string {
	c := ""
	if confirm != "" {
		c = `"confirm_password":"` + confirm + `",`
	}
	return `{"email":"pia@example.com","name":"Pia","password":"` + strongPassword + `",` + c + `"area_assignment":"Metro","church_name":"AMGC 1"` + extra + `}`
}

func TestAdminUserHandler_CreateUser(t *testing.T) {
	t.Run("201, passes the request and the session user to the service, never returns the hash", func(t *testing.T) {
		svc := &fakeUserSvc{}
		rec, _ := serveUserAdmin(t, svc, 99, http.MethodPost, "/admin/users", createBody(`,"role_id":7`))

		require.Equal(t, http.StatusCreated, rec.Code, rec.Body.String())
		assert.Equal(t, int64(99), svc.gotActor)
		assert.Equal(t, "10.1.2.3", svc.gotIP)
		assert.Equal(t, "pia@example.com", svc.gotCreate.Email)
		assert.Equal(t, "Metro", svc.gotCreate.AreaAssignment)
		require.NotNil(t, svc.gotCreate.RoleID)
		assert.Equal(t, int64(7), *svc.gotCreate.RoleID)

		var got map[string]any
		require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &got))
		assert.Equal(t, "pia@example.com", got["email"])
		assert.Equal(t, "Metro", got["area_assignment"])
		assert.NotContains(t, got, "worker", "an account has no separate worker; reports use the name")
		assert.NotContains(t, rec.Body.String(), "secret-hash")
		assert.NotContains(t, rec.Body.String(), "password")
	})

	validation := []struct {
		name, body, field string
	}{
		{"bad email", `{"email":"nope","name":"Pia","password":"` + strongPassword + `"}`, "email"},
		{"short name", `{"email":"a@example.com","name":"P","password":"` + strongPassword + `"}`, "name"},
		{"weak password", `{"email":"a@example.com","name":"Pia","password":"password"}`, "password"},
		{"missing password", `{"email":"a@example.com","name":"Pia"}`, "password"},
		{"zero role", createBody(`,"role_id":0`), "role_id"},
		{"area too long", strings.Replace(createBody(""), `"Metro"`, `"`+strings.Repeat("x", 151)+`"`, 1), "area_assignment"},
	}
	for _, tt := range validation {
		t.Run("422 "+tt.name, func(t *testing.T) {
			svc := &fakeUserSvc{}
			rec, eb := serveUserAdmin(t, svc, 99, http.MethodPost, "/admin/users", tt.body)
			assert.Equal(t, http.StatusUnprocessableEntity, rec.Code, rec.Body.String())
			assert.Contains(t, eb.Details, tt.field)
			assert.Zero(t, svc.calls, "an invalid request must not reach the service")
			assert.NotContains(t, rec.Body.String(), strongPassword, "details must not echo the password")
		})
	}

	confirm := []struct{ name, body, want string }{
		{"missing confirm_password", createBodyConfirm("", ""), "is required"},
		{"different confirm_password", createBodyConfirm(strongPassword+"x", ""), "does not match"},
		{"case-only difference", createBodyConfirm(strings.ToLower(strongPassword), ""), "does not match"},
		{"trailing space", createBodyConfirm(strongPassword+" ", ""), "does not match"},
	}
	for _, tt := range confirm {
		t.Run("422 "+tt.name, func(t *testing.T) {
			svc := &fakeUserSvc{}
			rec, eb := serveUserAdmin(t, svc, 99, http.MethodPost, "/admin/users", tt.body)
			assert.Equal(t, http.StatusUnprocessableEntity, rec.Code, rec.Body.String())
			assert.Equal(t, tt.want, eb.Details["confirm_password"])
			assert.Zero(t, svc.calls, "a mismatch must not reach the service")
			assert.NotContains(t, rec.Body.String(), strongPassword)
		})
	}

	t.Run("a weak password and a mismatch are both reported", func(t *testing.T) {
		rec, eb := serveUserAdmin(t, &fakeUserSvc{}, 99, http.MethodPost, "/admin/users",
			`{"email":"a@example.com","name":"Pia","password":"password","confirm_password":"other"}`)
		require.Equal(t, http.StatusUnprocessableEntity, rec.Code)
		assert.Contains(t, eb.Details, "password")
		assert.Equal(t, "does not match", eb.Details["confirm_password"])
	})

	t.Run("422 for an empty body names every required field", func(t *testing.T) {
		rec, eb := serveUserAdmin(t, &fakeUserSvc{}, 99, http.MethodPost, "/admin/users", `{}`)
		require.Equal(t, http.StatusUnprocessableEntity, rec.Code)
		for _, field := range []string{"email", "name", "password", "confirm_password"} {
			assert.Equal(t, "is required", eb.Details[field], field)
		}
	})

	failures := []struct {
		name string
		err  error
		want int
	}{
		{"email taken", user.ErrEmailTaken, http.StatusConflict},
		{"role not found", user.ErrRoleNotFound, http.StatusUnprocessableEntity},
		{"actor gone", auth.ErrUnauthorized, http.StatusUnauthorized},
		{"anything else is hidden", errors.New("db exploded"), http.StatusInternalServerError},
	}
	for _, tt := range failures {
		t.Run(tt.name, func(t *testing.T) {
			rec, eb := serveUserAdmin(t, &fakeUserSvc{err: tt.err}, 99, http.MethodPost, "/admin/users", createBody(""))
			assert.Equal(t, tt.want, rec.Code, rec.Body.String())
			assert.NotContains(t, eb.Message, "db exploded")
		})
	}

	t.Run("400 for a body that is not JSON", func(t *testing.T) {
		rec, _ := serveUserAdmin(t, &fakeUserSvc{}, 99, http.MethodPost, "/admin/users", "{nope")
		assert.Equal(t, http.StatusBadRequest, rec.Code)
	})

	t.Run("401 without a session user", func(t *testing.T) {
		svc := &fakeUserSvc{}
		rec, _ := serveUserAdmin(t, svc, 0, http.MethodPost, "/admin/users", createBody(""))
		assert.Equal(t, http.StatusUnauthorized, rec.Code)
		assert.Zero(t, svc.calls)
	})
}

func TestAdminUserHandler_UpdateUser(t *testing.T) {
	const body = `{"name":"New Name","area_assignment":"North","church_name":""}`

	t.Run("200, passes id, request and session user", func(t *testing.T) {
		svc := &fakeUserSvc{}
		rec, _ := serveUserAdmin(t, svc, 99, http.MethodPut, "/admin/users/12", body)
		require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
		assert.Equal(t, int64(12), svc.gotID)
		assert.Equal(t, int64(99), svc.gotActor)
		assert.Equal(t, "North", svc.gotUpdate.AreaAssignment)
		assert.NotContains(t, rec.Body.String(), "secret-hash")
	})

	t.Run("404 for an unknown user", func(t *testing.T) {
		rec, _ := serveUserAdmin(t, &fakeUserSvc{err: user.ErrUserNotFound}, 99, http.MethodPut, "/admin/users/12", body)
		assert.Equal(t, http.StatusNotFound, rec.Code)
	})

	t.Run("400 for a bad id", func(t *testing.T) {
		for _, id := range []string{"abc", "0", "-3"} {
			svc := &fakeUserSvc{}
			rec, _ := serveUserAdmin(t, svc, 99, http.MethodPut, "/admin/users/"+id, body)
			assert.Equal(t, http.StatusBadRequest, rec.Code, id)
			assert.Zero(t, svc.calls)
		}
	})

	t.Run("422 for a short name or long text", func(t *testing.T) {
		for field, bad := range map[string]string{
			"name":        `{"name":"P"}`,
			"church_name": `{"name":"Pia","church_name":"` + strings.Repeat("x", 151) + `"}`,
		} {
			svc := &fakeUserSvc{}
			rec, eb := serveUserAdmin(t, svc, 99, http.MethodPut, "/admin/users/12", bad)
			assert.Equal(t, http.StatusUnprocessableEntity, rec.Code)
			assert.Contains(t, eb.Details, field)
			assert.Zero(t, svc.calls)
		}
	})

	t.Run("401 without a session user, 500 for other errors", func(t *testing.T) {
		rec, _ := serveUserAdmin(t, &fakeUserSvc{}, 0, http.MethodPut, "/admin/users/12", body)
		assert.Equal(t, http.StatusUnauthorized, rec.Code)
		rec, _ = serveUserAdmin(t, &fakeUserSvc{err: errors.New("boom")}, 99, http.MethodPut, "/admin/users/12", body)
		assert.Equal(t, http.StatusInternalServerError, rec.Code)
	})
}
