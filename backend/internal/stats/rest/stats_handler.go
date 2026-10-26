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
// @Description  Counts for one month: report statuses, active expected workers, workers with no report, and unique active churches; plus all users and admins regardless of month. Needs tracker:read. Without month and year the period is the previous month in Asia/Manila. Month and year go together; the month is an English month name, case-insensitive.
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
// @Description  Every church account: the user role, not deleted, with a church name (any status: active, inactive, disciplinary, deceased; D-A24), with separate submission and review statuses for one month. Submission status is submitted or no_report; review status is approved, pending or needs_revision when a report exists. Ordered by church name, then name. No report text. Needs tracker:read. Without month and year the period is the previous month in Asia/Manila. Month and year go together; the month is an English month name, case-insensitive.
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

// GetAttendanceAnalytics godoc
// @Summary      Annual attendance analytics
// @Description  Weekly metric totals across the year and by church. Defaults to approved reports; status=all includes pending, needs_revision, and approved reports. Needs tracker:read.
// @Tags         admin/analytics
// @Produce      json
// @Security     CookieAuth
// @Param        year query int true "Year"
// @Param        metric query string true "Metric key"
// @Param        status query string false "approved or all; defaults to approved"
// @Success      200 {object} stats.AttendanceAnalytics
// @Failure      400 {object} map[string]string
// @Failure      401 {object} map[string]string
// @Failure      403 {object} map[string]string
// @Router       /admin/analytics/attendance [get]
func (h *StatsHandler) GetAttendanceAnalytics(c echo.Context) error {
	year, err := strconv.Atoi(c.QueryParam("year"))
	if err != nil || year <= 0 {
		return apperror.BadRequest("invalid year")
	}
	if c.QueryParam("metric") == "" {
		return apperror.BadRequest("invalid metric")
	}
	q := stats.AttendanceQuery{Year: year, Metric: c.QueryParam("metric"), Status: c.QueryParam("status")}
	out, err := h.svc.AnalyticsAttendance(c.Request().Context(), q)
	if errors.Is(err, stats.ErrAnalyticsQuery) {
		return apperror.BadRequest(stats.ErrAnalyticsQuery.Error())
	}
	if err != nil {
		return apperror.Internal("failed to get attendance analytics").WithError(err)
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
