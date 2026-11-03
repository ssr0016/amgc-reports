package rest

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/labstack/echo/v4"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ssr0016/pastoral-reports/backend/internal/apperror"
	"github.com/ssr0016/pastoral-reports/backend/internal/notification"
	"github.com/ssr0016/pastoral-reports/backend/internal/request"
)

type fakeStore struct {
	rows       []notification.Notification
	listUserID int64
	readUserID int64
	readID     int64
	readErr    error
}

func (f *fakeStore) CreateReportSubmitted(context.Context, int64, string) error { return nil }
func (f *fakeStore) ListUnread(_ context.Context, userID int64) ([]notification.Notification, error) {
	f.listUserID = userID
	return f.rows, nil
}
func (f *fakeStore) MarkRead(_ context.Context, userID, id int64) error {
	f.readUserID, f.readID = userID, id
	return f.readErr
}

func serveNotification(t *testing.T, store notification.Store, method, target string, userID int64) *httptest.ResponseRecorder {
	t.Helper()
	e := echo.New()
	e.HTTPErrorHandler = apperror.ErrorHandler
	h := NewNotificationHandler(store)
	e.GET("/api/v1/notifications", h.List)
	e.PATCH("/api/v1/notifications/:id/read", h.MarkRead)
	ctx := context.Background()
	if userID != 0 {
		ctx = request.WithUser(ctx, request.User{ID: userID})
	}
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, httptest.NewRequestWithContext(ctx, method, target, nil))
	return rec
}

func TestNotificationHandler_List(t *testing.T) {
	created := time.Date(2026, 11, 3, 8, 0, 0, 0, time.UTC)
	store := &fakeStore{rows: []notification.Notification{{
		ID: 7, UserID: 4, Type: "report_submitted", Message: "Report submitted by Ana",
		Metadata: map[string]any{"report_id": float64(42)}, CreatedAt: created,
	}}}
	rec := serveNotification(t, store, http.MethodGet, "/api/v1/notifications", 4)
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	assert.Equal(t, int64(4), store.listUserID)
	var got []notification.NotificationResponse
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &got))
	require.Len(t, got, 1)
	assert.Equal(t, int64(7), got[0].ID)
	assert.Equal(t, float64(42), got[0].Metadata["report_id"])
}

func TestNotificationHandler_ListEmptyIsArray(t *testing.T) {
	rec := serveNotification(t, &fakeStore{}, http.MethodGet, "/api/v1/notifications", 4)
	require.Equal(t, http.StatusOK, rec.Code)
	assert.Equal(t, "[]\n", rec.Body.String())
}

func TestNotificationHandler_MarkRead(t *testing.T) {
	store := &fakeStore{}
	rec := serveNotification(t, store, http.MethodPatch, "/api/v1/notifications/7/read", 4)
	require.Equal(t, http.StatusNoContent, rec.Code)
	assert.Equal(t, int64(4), store.readUserID)
	assert.Equal(t, int64(7), store.readID)

	for _, target := range []string{"/api/v1/notifications/0/read", "/api/v1/notifications/no/read"} {
		rec = serveNotification(t, store, http.MethodPatch, target, 4)
		assert.Equal(t, http.StatusBadRequest, rec.Code, target)
	}
}

func TestNotificationHandler_MarkReadNotFoundAndUnauthenticated(t *testing.T) {
	store := &fakeStore{readErr: notification.ErrNotFound}
	rec := serveNotification(t, store, http.MethodPatch, "/api/v1/notifications/7/read", 4)
	assert.Equal(t, http.StatusNotFound, rec.Code)

	for _, method := range []string{http.MethodGet, http.MethodPatch} {
		target := "/api/v1/notifications"
		if method == http.MethodPatch {
			target += "/7/read"
		}
		rec = serveNotification(t, store, method, target, 0)
		assert.Equal(t, http.StatusUnauthorized, rec.Code, method)
	}
}
