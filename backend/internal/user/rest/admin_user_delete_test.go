package rest

import (
	"net/http"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ssr0016/pastoral-reports/backend/internal/user"
)

func TestAdminUserHandler_DeleteUser(t *testing.T) {
	const url = "/admin/users/12"

	t.Run("200, passes the session user, the ip, the path id and the reason to the service", func(t *testing.T) {
		svc := &fakeUserSvc{}
		rec, _ := serveUserAdmin(t, svc, 99, http.MethodDelete, url, `{"reason":"left the church"}`)
		require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())

		assert.Equal(t, 1, svc.calls)
		assert.Equal(t, int64(99), svc.gotActor)
		assert.Equal(t, int64(12), svc.gotID)
		assert.Equal(t, "left the church", svc.gotReason)
		assert.Equal(t, "10.1.2.3", svc.gotIP)
		assert.Contains(t, rec.Body.String(), `"deleted_at"`)
		assert.NotContains(t, rec.Body.String(), "secret-hash")
	})

	t.Run("an empty body is allowed", func(t *testing.T) {
		svc := &fakeUserSvc{}
		rec, _ := serveUserAdmin(t, svc, 99, http.MethodDelete, url, "")
		require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
		assert.Empty(t, svc.gotReason)
	})

	t.Run("a reason over 500 characters is 422 on reason, and the service is not called", func(t *testing.T) {
		svc := &fakeUserSvc{}
		rec, eb := serveUserAdmin(t, svc, 99, http.MethodDelete, url, `{"reason":"`+strings.Repeat("x", 501)+`"}`)
		assert.Equal(t, http.StatusUnprocessableEntity, rec.Code)
		assert.Contains(t, eb.Details, "reason")
		assert.Zero(t, svc.calls)
	})

	t.Run("errors map to their status codes", func(t *testing.T) {
		tests := []struct {
			err     error
			code    int
			message string
		}{
			{user.ErrCannotDeleteSelf, http.StatusForbidden, "You cannot delete your own account."},
			{user.ErrLastActiveAdmin, http.StatusConflict, "At least one active admin is required."},
			{user.ErrUserNotFound, http.StatusNotFound, "user not found"},
		}
		for _, tt := range tests {
			rec, eb := serveUserAdmin(t, &fakeUserSvc{err: tt.err}, 99, http.MethodDelete, url, "")
			assert.Equal(t, tt.code, rec.Code, tt.err.Error())
			assert.Equal(t, tt.message, eb.Message, tt.err.Error())
		}
	})

	t.Run("a bad id is 400, no session is 401, a broken body is 400", func(t *testing.T) {
		svc := &fakeUserSvc{}
		rec, _ := serveUserAdmin(t, svc, 99, http.MethodDelete, "/admin/users/abc", "")
		assert.Equal(t, http.StatusBadRequest, rec.Code)
		rec, _ = serveUserAdmin(t, svc, 0, http.MethodDelete, url, "")
		assert.Equal(t, http.StatusUnauthorized, rec.Code)
		rec, _ = serveUserAdmin(t, svc, 99, http.MethodDelete, url, `{nope`)
		assert.Equal(t, http.StatusBadRequest, rec.Code)
		assert.Zero(t, svc.calls)
	})
}

func TestAdminUserHandler_RestoreUser(t *testing.T) {
	t.Run("200 without deleted_at, passes the session user and the path id", func(t *testing.T) {
		svc := &fakeUserSvc{}
		rec, _ := serveUserAdmin(t, svc, 99, http.MethodPost, "/admin/users/12/restore", "")
		require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
		assert.Equal(t, int64(99), svc.gotActor)
		assert.Equal(t, int64(12), svc.gotID)
		assert.NotContains(t, rec.Body.String(), "deleted_at")
	})

	t.Run("a bad id is 400 and a missing user is 404", func(t *testing.T) {
		svc := &fakeUserSvc{}
		rec, _ := serveUserAdmin(t, svc, 99, http.MethodPost, "/admin/users/0/restore", "")
		assert.Equal(t, http.StatusBadRequest, rec.Code)
		assert.Zero(t, svc.calls)
		rec, _ = serveUserAdmin(t, &fakeUserSvc{err: user.ErrUserNotFound}, 99, http.MethodPost, "/admin/users/12/restore", "")
		assert.Equal(t, http.StatusNotFound, rec.Code)
	})
}

// Creating with a deleted user's email points to Restore (D-A11).
func TestAdminUserHandler_CreateUser_DeletedEmailIs409(t *testing.T) {
	rec, eb := serveUserAdmin(t, &fakeUserSvc{err: user.ErrEmailBelongsToDeletedUser}, 99, http.MethodPost, "/admin/users", createBody(""))
	assert.Equal(t, http.StatusConflict, rec.Code)
	assert.Equal(t, "This email belongs to a deleted user. Restore them instead.", eb.Message)
}
