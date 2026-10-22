// Package rest serves the audit trail to admins, read-only.
package rest

import (
	"errors"
	"net/http"
	"strconv"

	"github.com/labstack/echo/v4"

	"github.com/ssr0016/pastoral-reports/backend/internal/apperror"
	"github.com/ssr0016/pastoral-reports/backend/internal/audit"
	"github.com/ssr0016/pastoral-reports/backend/pkg/pagination"
)

// pageSize is the Logs tab's page; other lists keep pagination.DefaultLimit.
const pageSize = 50

type AuditHandler struct {
	svc audit.Service
}

func NewAuditHandler(svc audit.Service) *AuditHandler { return &AuditHandler{svc: svc} }

// List godoc
// @Summary      Audit log
// @Description  Admin role only. One page of the audit trail for one month in Asia/Manila, newest first, with the total. Optionally one action. Read-only: no endpoint changes or deletes a row. The IP address is never returned (D-A4). Details hold only the allow-listed keys of each action, never a password, token or report text.
// @Tags         admin/audit
// @Produce      json
// @Security     CookieAuth
// @Param        month query string true "Month name, case-insensitive"
// @Param        year query int true "Year, 2000 to 2100"
// @Param        action query string false "One audit action, e.g. LOGIN_FAILED"
// @Param        page query int false "Page, from 1" default(1)
// @Param        limit query int false "Rows per page, at most 100" default(50)
// @Success      200 {object} pagination.Response[audit.LogResponse]
// @Failure      400 {object} map[string]string
// @Failure      401 {object} map[string]string
// @Failure      403 {object} map[string]string
// @Failure      422 {object} map[string]string
// @Failure      429 {object} map[string]string
// @Router       /admin/audit-logs [get]
func (h *AuditHandler) List(c echo.Context) error {
	q := audit.ListQuery{Month: c.QueryParam("month")}
	if y := c.QueryParam("year"); y != "" {
		n, err := strconv.Atoi(y)
		if err != nil {
			return apperror.BadRequest("invalid year")
		}
		q.Year = n
	}
	if a := c.QueryParam("action"); a != "" {
		q.Action = &a
	}
	params := pagination.FromQuery(c.QueryParams())
	if c.QueryParam("limit") == "" {
		params.Limit = pageSize
	}

	rows, total, err := h.svc.List(c.Request().Context(), q, params)
	switch {
	case errors.Is(err, audit.ErrPeriod):
		msg := audit.ErrPeriod.Error()
		return apperror.Validation(msg).WithDetails(map[string]string{"month": msg})
	case errors.Is(err, audit.ErrAction):
		msg := audit.ErrAction.Error()
		return apperror.Validation(msg).WithDetails(map[string]string{"action": msg})
	case err != nil:
		return apperror.Internal("failed to list audit logs").WithError(err)
	}
	out := make([]audit.LogResponse, 0, len(rows))
	for _, r := range rows {
		out = append(out, r.ToResponse())
	}
	return c.JSON(http.StatusOK, pagination.NewResponse(out, params, total))
}
