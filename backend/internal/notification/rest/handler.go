// Package rest serves user-scoped in-app notifications.
package rest

import (
	"errors"
	"net/http"
	"strconv"

	"github.com/labstack/echo/v4"

	"github.com/ssr0016/pastoral-reports/backend/internal/apperror"
	"github.com/ssr0016/pastoral-reports/backend/internal/notification"
	"github.com/ssr0016/pastoral-reports/backend/internal/request"
)

type NotificationHandler struct{ store notification.Store }

func NewNotificationHandler(store notification.Store) *NotificationHandler {
	return &NotificationHandler{store: store}
}

func (h *NotificationHandler) userID(c echo.Context) (int64, error) {
	u, ok := request.UserFrom(c.Request().Context())
	if !ok {
		return 0, apperror.Unauthorized("authentication required")
	}
	return u.ID, nil
}

// List godoc
// @Summary      List unread notifications
// @Description  Returns unread notifications for the authenticated user, newest first.
// @Tags         notifications
// @Produce      json
// @Security     CookieAuth
// @Success      200 {array} notification.NotificationResponse
// @Failure      401 {object} map[string]string
// @Failure      500 {object} map[string]string
// @Router       /notifications [get]
func (h *NotificationHandler) List(c echo.Context) error {
	uid, err := h.userID(c)
	if err != nil {
		return err
	}
	rows, err := h.store.ListUnread(c.Request().Context(), uid)
	if err != nil {
		return apperror.Internal("failed to list notifications").WithError(err)
	}
	out := make([]notification.NotificationResponse, 0, len(rows))
	for _, row := range rows {
		out = append(out, row.ToResponse())
	}
	return c.JSON(http.StatusOK, out)
}

// MarkRead godoc
// @Summary      Mark notification as read
// @Description  Marks one notification as read for the authenticated user.
// @Tags         notifications
// @Security     CookieAuth
// @Param        id path int true "Notification ID"
// @Success      204
// @Failure      400 {object} map[string]string
// @Failure      401 {object} map[string]string
// @Failure      404 {object} map[string]string
// @Failure      500 {object} map[string]string
// @Router       /notifications/{id}/read [patch]
func (h *NotificationHandler) MarkRead(c echo.Context) error {
	uid, err := h.userID(c)
	if err != nil {
		return err
	}
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil || id <= 0 {
		return apperror.BadRequest("invalid notification id")
	}
	if err := h.store.MarkRead(c.Request().Context(), uid, id); err != nil {
		if errors.Is(err, notification.ErrNotFound) {
			return apperror.NotFound("notification not found")
		}
		return apperror.Internal("failed to mark notification read").WithError(err)
	}
	return c.NoContent(http.StatusNoContent)
}
