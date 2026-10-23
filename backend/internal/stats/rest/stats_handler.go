package rest

import (
	"errors"
	"net/http"
	"strconv"

	"github.com/labstack/echo/v4"

	"github.com/ssr0016/pastoral-reports/backend/internal/apperror"
	"github.com/ssr0016/pastoral-reports/backend/internal/stats"
)

type StatsHandler struct {
	svc stats.Service
}

func NewStatsHandler(svc stats.Service) *StatsHandler { return &StatsHandler{svc: svc} }

// GetStats godoc
// @Summary      Admin panel counts
// @Description  Counts for one month: reports (total, approved, pending) and, regardless of month, all users and admins. Needs tracker:read. Without month and year the period is the previous month in Asia/Manila. Month and year go together; the month is an English month name, case-insensitive.
// @Tags         admin/stats
// @Produce      json
// @Security     CookieAuth
// @Param        month query string false "Month name, case-insensitive; requires year"
// @Param        year query int false "Year; requires month"
// @Success      200 {object} stats.AdminStats
// @Failure      400 {object} map[string]string
// @Failure      401 {object} map[string]string
// @Failure      403 {object} map[string]string
// @Failure      422 {object} map[string]string
// @Router       /admin/stats [get]
func (h *StatsHandler) GetStats(c echo.Context) error {
	q, err := periodQuery(c)
	if err != nil {
		return err
	}
	out, err := h.svc.Get(c.Request().Context(), q)
	switch {
	case errors.Is(err, stats.ErrPeriod):
		return periodError()
	case err != nil:
		return apperror.Internal("failed to get stats").WithError(err)
	}
	return c.JSON(http.StatusOK, out)
}

// GetTracker godoc
// @Summary      Who has reported this month
// @Description  Every church account: the user role, not deleted, with a church name (any status: active, inactive, disciplinary, deceased; D-A24), with its report status for one month: approved, for_review (filed, not approved) or not_submitted (D-A3). Ordered by church name, then name. No report text. Needs tracker:read. Without month and year the period is the previous month in Asia/Manila. Month and year go together; the month is an English month name, case-insensitive.
// @Tags         admin/stats
// @Produce      json
// @Security     CookieAuth
// @Param        month query string false "Month name, case-insensitive; requires year"
// @Param        year query int false "Year; requires month"
// @Success      200 {object} stats.Tracker
// @Failure      400 {object} map[string]string
// @Failure      401 {object} map[string]string
// @Failure      403 {object} map[string]string
// @Failure      422 {object} map[string]string
// @Router       /admin/tracker [get]
func (h *StatsHandler) GetTracker(c echo.Context) error {
	q, err := periodQuery(c)
	if err != nil {
		return err
	}
	out, err := h.svc.Tracker(c.Request().Context(), q)
	switch {
	case errors.Is(err, stats.ErrPeriod):
		return periodError()
	case err != nil:
		return apperror.Internal("failed to get tracker").WithError(err)
	}
	return c.JSON(http.StatusOK, out)
}

// periodQuery reads ?month= and ?year=; an empty value is the same as none.
func periodQuery(c echo.Context) (stats.GetStatsQuery, error) {
	var q stats.GetStatsQuery
	if m := c.QueryParam("month"); m != "" {
		q.Month = &m
	}
	if y := c.QueryParam("year"); y != "" {
		n, err := strconv.Atoi(y)
		if err != nil || n <= 0 {
			return q, apperror.BadRequest("invalid year")
		}
		q.Year = &n
	}
	return q, nil
}

func periodError() error {
	msg := stats.ErrPeriod.Error()
	return apperror.Validation(msg).WithDetails(map[string]string{"month": msg})
}
