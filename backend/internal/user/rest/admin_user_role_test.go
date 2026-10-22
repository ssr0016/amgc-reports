package rest

import (
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ssr0016/pastoral-reports/backend/internal/user"
)

func TestAdminUserHandler_ChangeRole(t *testing.T) {
	const url = "/admin/users/12/role"

	t.Run("200, passes the session user, the ip, the path id and the role to the service", func(t *testing.T) {
		svc := &fakeUserSvc{}
		rec, _ := serveUserAdmin(t, svc, 99, http.MethodPut, url, `{"role_id":4}`)
		require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())

		assert.Equal(t, int64(99), svc.gotActor)
		assert.Equal(t, int64(12), svc.gotID)
		assert.Equal(t, int64(4), svc.gotRoleID)
		assert.Equal(t, "10.1.2.3", svc.gotIP)
		assert.NotContains(t, rec.Body.String(), "secret-hash")
	})

	t.Run("a missing role_id is 422 and the service is not called", func(t *testing.T) {
		svc := &fakeUserSvc{}
		rec, eb := serveUserAdmin(t, svc, 99, http.MethodPut, url, `{}`)
		assert.Equal(t, http.StatusUnprocessableEntity, rec.Code)
		assert.Contains(t, eb.Details, "role_id")
		assert.Zero(t, svc.calls)
	})

	t.Run("errors map to their status codes", func(t *testing.T) {
		tests := []struct {
			err     error
			code    int
			message string
		}{
			{user.ErrCannotChangeOwnRole, http.StatusForbidden, "You cannot change your own role."},
			{user.ErrLastActiveAdmin, http.StatusConflict, "At least one active admin is required."},
			{user.ErrRoleNotFound, http.StatusNotFound, "role not found"},
			{user.ErrUserNotFound, http.StatusNotFound, "user not found"},
		}
		for _, tt := range tests {
			rec, eb := serveUserAdmin(t, &fakeUserSvc{err: tt.err}, 99, http.MethodPut, url, `{"role_id":4}`)
			assert.Equal(t, tt.code, rec.Code, tt.err)
			assert.Equal(t, tt.message, eb.Message, tt.err)
		}
	})
}
