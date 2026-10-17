package handler

import (
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ssr0016/pastoral-reports/backend/internal/service"
)

func TestAdminUserHandler_ChangeStatus(t *testing.T) {
	const url = "/admin/users/12/status"

	t.Run("200, passes the session user, the ip, the path id and the status to the service", func(t *testing.T) {
		svc := &fakeUserSvc{}
		rec, _ := serveUserAdmin(t, svc, 99, http.MethodPut, url, `{"status":"inactive"}`)
		require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())

		assert.Equal(t, 1, svc.calls)
		assert.Equal(t, int64(99), svc.gotActor)
		assert.Equal(t, int64(12), svc.gotID)
		assert.Equal(t, "inactive", svc.gotStatus)
		assert.Equal(t, "10.1.2.3", svc.gotIP)
		assert.Contains(t, rec.Body.String(), `"status":"inactive"`)
		assert.NotContains(t, rec.Body.String(), "secret-hash")
	})

	t.Run("the actor comes from the session, never the body", func(t *testing.T) {
		svc := &fakeUserSvc{}
		serveUserAdmin(t, svc, 99, http.MethodPut, url, `{"status":"inactive","actor_id":1}`)
		assert.Equal(t, int64(99), svc.gotActor)
	})

	t.Run("a status outside the four is 422 on status, and the service is not called", func(t *testing.T) {
		for _, body := range []string{`{"status":"retired"}`, `{"status":""}`, `{}`} {
			svc := &fakeUserSvc{}
			rec, eb := serveUserAdmin(t, svc, 99, http.MethodPut, url, body)
			assert.Equal(t, http.StatusUnprocessableEntity, rec.Code, body)
			assert.Contains(t, eb.Details, "status", body)
			assert.Zero(t, svc.calls, body)
		}
	})

	t.Run("errors map to their status codes", func(t *testing.T) {
		tests := []struct {
			err     error
			code    int
			message string
		}{
			{service.ErrCannotChangeOwnStatus, http.StatusForbidden, "You cannot change your own status."},
			{service.ErrLastActiveAdmin, http.StatusConflict, "At least one active admin is required."},
			{service.ErrUserNotFound, http.StatusNotFound, "user not found"},
			{service.ErrInvalidUserStatus, http.StatusUnprocessableEntity, ""},
			{service.ErrUnauthorized, http.StatusUnauthorized, ""},
		}
		for _, tt := range tests {
			rec, eb := serveUserAdmin(t, &fakeUserSvc{err: tt.err}, 99, http.MethodPut, url, `{"status":"inactive"}`)
			assert.Equal(t, tt.code, rec.Code, tt.err.Error())
			if tt.message != "" {
				assert.Equal(t, tt.message, eb.Message, tt.err.Error())
			}
		}
	})

	t.Run("a bad id is 400, no session is 401, a broken body is 400", func(t *testing.T) {
		svc := &fakeUserSvc{}
		rec, _ := serveUserAdmin(t, svc, 99, http.MethodPut, "/admin/users/abc/status", `{"status":"inactive"}`)
		assert.Equal(t, http.StatusBadRequest, rec.Code)
		rec, _ = serveUserAdmin(t, svc, 0, http.MethodPut, url, `{"status":"inactive"}`)
		assert.Equal(t, http.StatusUnauthorized, rec.Code)
		rec, _ = serveUserAdmin(t, svc, 99, http.MethodPut, url, `{nope`)
		assert.Equal(t, http.StatusBadRequest, rec.Code)
		assert.Zero(t, svc.calls)
	})
}
