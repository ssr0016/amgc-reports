package handler

import (
	"encoding/json"
	"net/http"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ssr0016/pastoral-reports/backend/internal/service"
)

func resetBody(password, confirm string) string {
	m := map[string]string{"new_password": password}
	if confirm != "" {
		m["confirm_password"] = confirm
	}
	b, _ := json.Marshal(m)
	return string(b)
}

func TestAdminUserHandler_ResetPassword(t *testing.T) {
	const url = "/admin/users/12/reset-password"
	good := resetBody(strongPassword, strongPassword)

	t.Run("204 with no body, and the service gets the session user, the ip, the path id and the password", func(t *testing.T) {
		svc := &fakeUserSvc{}
		rec, _ := serveUserAdmin(t, svc, 99, http.MethodPost, url, good)

		require.Equal(t, http.StatusNoContent, rec.Code, rec.Body.String())
		assert.Empty(t, rec.Body.String(), "the reset answers with nothing: no hash, no password, no user")
		assert.Equal(t, 1, svc.calls)
		assert.Equal(t, int64(99), svc.gotActor)
		assert.Equal(t, int64(12), svc.gotID)
		assert.Equal(t, "10.1.2.3", svc.gotIP)
		assert.Equal(t, strongPassword, svc.gotPassword)
	})

	t.Run("the actor comes from the session, never the body", func(t *testing.T) {
		svc := &fakeUserSvc{}
		body := `{"new_password":"` + strongPassword + `","confirm_password":"` + strongPassword + `","actor_id":1,"id":1}`
		serveUserAdmin(t, svc, 99, http.MethodPost, url, body)
		assert.Equal(t, int64(99), svc.gotActor)
		assert.Equal(t, int64(12), svc.gotID)
	})

	t.Run("a weak, mismatched or missing password is 422 on its field, the service is not called, the password is not echoed", func(t *testing.T) {
		tests := []struct{ name, body, field, want string }{
			{"weak", resetBody("password", "password"), "new_password", "is too weak; use a longer, more varied password"},
			{"missing new password", `{"confirm_password":"x"}`, "new_password", "is required"},
			{"missing confirmation", resetBody(strongPassword, ""), "confirm_password", "is required"},
			{"different confirmation", resetBody(strongPassword, strongPassword+"x"), "confirm_password", "does not match"},
			{"case-only difference", resetBody(strongPassword, strings.ToLower(strongPassword)), "confirm_password", "does not match"},
			{"trailing space", resetBody(strongPassword, strongPassword+" "), "confirm_password", "does not match"},
		}
		for _, tt := range tests {
			svc := &fakeUserSvc{}
			rec, eb := serveUserAdmin(t, svc, 99, http.MethodPost, url, tt.body)
			assert.Equal(t, http.StatusUnprocessableEntity, rec.Code, tt.name)
			assert.Equal(t, tt.want, eb.Details[tt.field], tt.name)
			assert.Zero(t, svc.calls, tt.name)
			assert.NotContains(t, rec.Body.String(), strongPassword, tt.name)
		}
	})

	t.Run("a weak password and a mismatch are both reported", func(t *testing.T) {
		rec, eb := serveUserAdmin(t, &fakeUserSvc{}, 99, http.MethodPost, url, resetBody("password", "other"))
		require.Equal(t, http.StatusUnprocessableEntity, rec.Code)
		assert.Contains(t, eb.Details, "new_password")
		assert.Equal(t, "does not match", eb.Details["confirm_password"])
	})

	t.Run("validation answers before the self check: a self reset with a bad body is 422, not 403", func(t *testing.T) {
		svc := &fakeUserSvc{err: service.ErrCannotResetOwnPassword}
		rec, _ := serveUserAdmin(t, svc, 99, http.MethodPost, url, resetBody("password", "password"))
		assert.Equal(t, http.StatusUnprocessableEntity, rec.Code)
		assert.Zero(t, svc.calls)
	})

	t.Run("errors map to their status codes and messages", func(t *testing.T) {
		tests := []struct {
			err     error
			code    int
			message string
			field   string
		}{
			{service.ErrCannotResetOwnPassword, http.StatusForbidden, "You cannot reset your own password. Use the Change Password page instead.", ""},
			{service.ErrUserNotFound, http.StatusNotFound, "user not found", ""},
			{service.ErrUnauthorized, http.StatusUnauthorized, "", ""},
			{service.ErrPasswordTooLong, http.StatusUnprocessableEntity, "", "new_password"},
		}
		for _, tt := range tests {
			rec, eb := serveUserAdmin(t, &fakeUserSvc{err: tt.err}, 99, http.MethodPost, url, good)
			assert.Equal(t, tt.code, rec.Code, tt.err.Error())
			if tt.message != "" {
				assert.Equal(t, tt.message, eb.Message, tt.err.Error())
			}
			if tt.field != "" {
				assert.Equal(t, "must be at most 72 bytes", eb.Details[tt.field], tt.err.Error())
			}
		}
	})

	t.Run("an unexpected error is a 500 that hides its cause and the password", func(t *testing.T) {
		rec, eb := serveUserAdmin(t, &fakeUserSvc{err: assert.AnError}, 99, http.MethodPost, url, good)
		assert.Equal(t, http.StatusInternalServerError, rec.Code)
		assert.NotContains(t, eb.Message, assert.AnError.Error())
		assert.NotContains(t, rec.Body.String(), strongPassword)
	})

	t.Run("a bad id is 400, no session is 401, a broken body is 400, and none reach the service", func(t *testing.T) {
		svc := &fakeUserSvc{}
		for _, bad := range []string{"abc", "0", "-3"} {
			rec, _ := serveUserAdmin(t, svc, 99, http.MethodPost, "/admin/users/"+bad+"/reset-password", good)
			assert.Equal(t, http.StatusBadRequest, rec.Code, bad)
		}
		rec, _ := serveUserAdmin(t, svc, 0, http.MethodPost, url, good)
		assert.Equal(t, http.StatusUnauthorized, rec.Code)
		rec, _ = serveUserAdmin(t, svc, 99, http.MethodPost, url, `{nope`)
		assert.Equal(t, http.StatusBadRequest, rec.Code)
		assert.Zero(t, svc.calls)
	})
}
