package handler

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/alexedwards/scs/v2"
	"github.com/labstack/echo/v4"

	"github.com/ssr0016/pastoral-reports/backend/internal/apperror"
	"github.com/ssr0016/pastoral-reports/backend/internal/model"
	"github.com/ssr0016/pastoral-reports/backend/internal/repository"
	"github.com/ssr0016/pastoral-reports/backend/internal/service"
)

type meRoles struct {
	repository.RoleRepository
	perms []string
}

func (m meRoles) GetPermissions(context.Context, int64) ([]model.Permission, error) {
	var out []model.Permission
	for _, n := range m.perms {
		out = append(out, model.Permission{Name: n})
	}
	return out, nil
}

// serveMe calls GET /auth/me as the given session user (0 = no session).
func serveMe(t *testing.T, users *repository.MockUserRepo, perms []string, userID int64) *httptest.ResponseRecorder {
	t.Helper()
	sm := scs.New()
	h := NewAuthHandler(service.NewAuthService(users, meRoles{perms: perms}, nil, nil, nil, nil), sm)

	e := echo.New()
	e.HTTPErrorHandler = apperror.ErrorHandler
	e.Use(echo.WrapMiddleware(sm.LoadAndSave))
	e.Use(func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			if userID != 0 {
				sm.Put(c.Request().Context(), "user_id", userID)
			}
			return next(c)
		}
	})
	e.GET("/auth/me", h.Me)

	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/auth/me", nil))
	return rec
}

func TestAuthHandler_Me(t *testing.T) {
	users := repository.NewMockUserRepo()
	users.GetWithRoleFunc = func(_ context.Context, id int64) (*model.User, error) {
		if id == 7 {
			return &model.User{ID: 7, Name: "Ed", Email: "ed@example.org", RoleID: 2, Role: &model.Role{Name: "editor"}}, nil
		}
		return nil, nil
	}

	t.Run("returns user_id, name, email, role and permissions", func(t *testing.T) {
		rec := serveMe(t, users, []string{"reports:read", "reports:write"}, 7)
		if rec.Code != http.StatusOK {
			t.Fatalf("status = %d, want 200 (%s)", rec.Code, rec.Body.String())
		}
		var got map[string]any
		if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
			t.Fatal(err)
		}
		if len(got) != 7 || got["user_id"] != float64(7) || got["name"] != "Ed" ||
			got["email"] != "ed@example.org" || got["role"] != "editor" {
			t.Errorf("body = %s", rec.Body.String())
		}
		// The profile keys are always there, empty when the admin has not set them.
		for _, key := range []string{"area_assignment", "church_name"} {
			if v, ok := got[key]; !ok || v != "" {
				t.Errorf("%s = %v (present: %v), want an empty string", key, v, ok)
			}
		}
		// The worker on a report is the user's name, so /auth/me has no separate worker.
		if _, ok := got["worker"]; ok {
			t.Errorf("body has a worker key: %s", rec.Body.String())
		}
		perms, _ := got["permissions"].([]any)
		if len(perms) != 2 || perms[0] != "reports:read" || perms[1] != "reports:write" {
			t.Errorf("permissions = %v", got["permissions"])
		}
	})

	t.Run("an empty permission list is [] not null", func(t *testing.T) {
		rec := serveMe(t, users, nil, 7)
		if want := `"permissions":[]`; !strings.Contains(rec.Body.String(), want) {
			t.Errorf("body = %s, want it to contain %s", rec.Body.String(), want)
		}
	})

	t.Run("no session is 401", func(t *testing.T) {
		if rec := serveMe(t, users, nil, 0); rec.Code != http.StatusUnauthorized {
			t.Errorf("status = %d, want 401", rec.Code)
		}
	})

	t.Run("a session user that no longer exists is 401, not 200", func(t *testing.T) {
		if rec := serveMe(t, users, nil, 99); rec.Code != http.StatusUnauthorized {
			t.Errorf("status = %d, want 401", rec.Code)
		}
	})
}
