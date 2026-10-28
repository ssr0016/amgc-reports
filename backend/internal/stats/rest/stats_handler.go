package rest

import (
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"strings"

	"github.com/labstack/echo/v4"

	"github.com/ssr0016/pastoral-reports/backend/internal/apperror"
	"github.com/ssr0016/pastoral-reports/backend/internal/request"
	"github.com/ssr0016/pastoral-reports/backend/internal/stats"
)

type StatsHandler struct {
	svc stats.Service
}

func NewStatsHandler(svc stats.Service) *StatsHandler { return &StatsHandler{svc: svc} }

// GetStats godoc
// @Summary      Admin panel counts
// @Description  Counts for one month: report statuses, active expected workers, workers with no report, and unique active churches; plus all users and admins regardless of month, and the accounts per role (every role) and per status (every status). Needs tracker:read. Without month and year the period is the previous month in Asia/Manila. Month and year go together; the month is an English month name, case-insensitive.
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
// @Summary      Period analytics
// @Description  Every metric's period figures with recorded-entry counts and coverage, the selected metric by month, week slot and church, and the church-by-month submission compliance (D-A31). Only months that have ended are counted; compare=true adds the same months of the previous year. Defaults to approved reports; status=all includes pending, needs_revision and approved. Compliance ignores status. Needs tracker:read.
// @Tags         admin/analytics
// @Produce      json
// @Security     CookieAuth
// @Param        year query int true "Year"
// @Param        metric query string true "Metric key"
// @Param        status query string false "approved or all; defaults to approved"
// @Param        period query string false "full_year, q1, q2, q3, q4, or a lowercase month; defaults to full_year"
// @Param        compare query bool false "Include the previous year's same period"
// @Param        churches query string false "Comma-separated church ids; empty means every church"
// @Success      200 {object} stats.AttendanceAnalytics
// @Failure      400 {object} map[string]string
// @Failure      401 {object} map[string]string
// @Failure      403 {object} map[string]string
// @Router       /admin/analytics/attendance [get]
func (h *StatsHandler) GetAttendanceAnalytics(c echo.Context) error {
	q, err := attendanceQuery(c)
	if err != nil {
		return err
	}
	out, err := h.svc.AnalyticsAttendance(c.Request().Context(), q)
	if errors.Is(err, stats.ErrAnalyticsQuery) {
		return apperror.BadRequest(stats.ErrAnalyticsQuery.Error())
	}
	if err != nil {
		return apperror.Internal("failed to get attendance analytics").WithError(err)
	}
	return c.JSON(http.StatusOK, out)
}

const xlsxMIME = "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet"

// ExportAttendanceAnalytics godoc
// @Summary      Period analytics as Excel
// @Description  The same query and figures as GET /admin/analytics/attendance as a formatted .xlsx: Full Metrics Summary, Summary, Monthly Totals, Weekly Trend, Church Breakdown, Metric Detail, Weekly Detail, Submission Compliance and Notes. Needs tracker:read. Writes no audit row. Rate limited with the other Excel downloads, 5 per minute per user.
// @Tags         admin/analytics
// @Produce      application/vnd.openxmlformats-officedocument.spreadsheetml.sheet
// @Security     CookieAuth
// @Param        year query int true "Year"
// @Param        metric query string true "Metric key"
// @Param        status query string false "approved or all; defaults to approved"
// @Param        period query string false "full_year, q1, q2, q3, q4, or a lowercase month; defaults to full_year"
// @Param        compare query bool false "Include the previous year's same period"
// @Param        churches query string false "Comma-separated church ids; empty means every church"
// @Success      200 {file} file
// @Failure      400 {object} map[string]string
// @Failure      401 {object} map[string]string
// @Failure      403 {object} map[string]string
// @Failure      429 {object} map[string]string
// @Router       /admin/analytics/attendance/excel [get]
func (h *StatsHandler) ExportAttendanceAnalytics(c echo.Context) error {
	q, err := attendanceQuery(c)
	if err != nil {
		return err
	}
	data, err := h.svc.AnalyticsExcel(c.Request().Context(), q)
	if errors.Is(err, stats.ErrAnalyticsQuery) {
		return apperror.BadRequest(stats.ErrAnalyticsQuery.Error())
	}
	if err != nil {
		return apperror.Internal("failed to export attendance analytics").WithError(err)
	}
	return download(c, stats.ExportFilename(q, "analytics", "xlsx"), xlsxMIME, data)
}

// download sends data as an attachment; the service accepted the query, so the name is plain ASCII.
func download(c echo.Context, filename, mime string, data []byte) error {
	c.Response().Header().Set(echo.HeaderContentDisposition, fmt.Sprintf(`attachment; filename="%s"`, filename))
	c.Response().Header().Set("Cache-Control", "no-store")
	return c.Blob(http.StatusOK, mime, data)
}

// ExportAnalyticsCSV godoc
// @Summary      Period analytics as CSV
// @Description  One flat table under the analytics query: kind=weekly_detail is one row per report, metric and week slot (every status, with whether the row is in the selected analytics); kind=compliance is one row per church and month of the period. UTF-8 with a byte-order mark; numbers plain, missing values empty with a state column. Needs tracker:read. Writes no audit row. Rate limited with the Excel downloads.
// @Tags         admin/analytics
// @Produce      text/csv
// @Security     CookieAuth
// @Param        kind query string true "weekly_detail or compliance"
// @Param        year query int true "Year"
// @Param        metric query string true "Metric key"
// @Param        status query string false "approved or all; defaults to approved"
// @Param        period query string false "full_year, q1, q2, q3, q4, or a lowercase month; defaults to full_year"
// @Param        compare query bool false "Include the previous year's same period"
// @Param        churches query string false "Comma-separated church ids; empty means every church"
// @Success      200 {file} file
// @Failure      400 {object} map[string]string
// @Failure      401 {object} map[string]string
// @Failure      403 {object} map[string]string
// @Failure      429 {object} map[string]string
// @Router       /admin/analytics/csv [get]
func (h *StatsHandler) ExportAnalyticsCSV(c echo.Context) error {
	q, err := attendanceQuery(c)
	if err != nil {
		return err
	}
	kind := c.QueryParam("kind")
	data, err := h.svc.AnalyticsCSV(c.Request().Context(), q, kind)
	if errors.Is(err, stats.ErrAnalyticsQuery) {
		return apperror.BadRequest(stats.ErrAnalyticsQuery.Error())
	}
	if err != nil {
		return apperror.Internal("failed to export analytics").WithError(err)
	}
	return download(c, stats.ExportFilename(q, kind, "csv"), "text/csv; charset=utf-8", data)
}

// SetChurchReporting godoc
// @Summary      Override a church's reporting months
// @Description  Optional per-church exceptions to the organization-wide start month (D-A32), as activity months ("YYYY-MM") on the church record, not a worker account. An empty start_month uses the organization default; an empty end_month is open-ended. The end must not be before the church's start. Needs users:write. Audits UPDATE_CHURCH_REPORTING.
// @Tags         admin/analytics
// @Accept       json
// @Produce      json
// @Security     CookieAuth
// @Param        id path int true "Church ID"
// @Param        body body stats.ChurchReportingRequest true "Reporting months"
// @Success      200 {object} stats.ChurchScope
// @Failure      400 {object} map[string]string
// @Failure      401 {object} map[string]string
// @Failure      403 {object} map[string]string
// @Failure      404 {object} map[string]string
// @Failure      422 {object} map[string]string
// @Router       /admin/churches/{id}/reporting [put]
func (h *StatsHandler) SetChurchReporting(c echo.Context) error {
	actor, ok := request.UserFrom(c.Request().Context())
	if !ok {
		return apperror.Unauthorized("authentication required")
	}
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil || id <= 0 {
		return apperror.BadRequest("invalid church id")
	}
	var req stats.ChurchReportingRequest
	if err := c.Bind(&req); err != nil {
		return apperror.BadRequest("invalid request body").WithError(err)
	}
	if err := c.Validate(&req); err != nil {
		return err
	}
	out, err := h.svc.SetChurchReporting(c.Request().Context(), actor.ID, c.RealIP(), id, req)
	switch {
	case errors.Is(err, stats.ErrChurchNotFound):
		return apperror.NotFound(err.Error())
	case errors.Is(err, stats.ErrChurchPeriod):
		return apperror.Validation(err.Error())
	case err != nil:
		return apperror.Internal("failed to set church reporting").WithError(err)
	}
	return c.JSON(http.StatusOK, out)
}

// attendanceQuery reads the analytics query; the service checks metric, status and period.
func attendanceQuery(c echo.Context) (stats.AttendanceQuery, error) {
	year, err := strconv.Atoi(c.QueryParam("year"))
	if err != nil || year <= 0 {
		return stats.AttendanceQuery{}, apperror.BadRequest("invalid year")
	}
	if c.QueryParam("metric") == "" {
		return stats.AttendanceQuery{}, apperror.BadRequest("invalid metric")
	}
	period := c.QueryParam("period")
	if period == "" {
		period = stats.AttendancePeriodFullYear
	}
	compare := false
	if raw := c.QueryParam("compare"); raw != "" {
		compare, err = strconv.ParseBool(raw)
		if err != nil {
			return stats.AttendanceQuery{}, apperror.BadRequest("invalid compare")
		}
	}
	var churches []int64
	for _, raw := range strings.Split(c.QueryParam("churches"), ",") {
		if raw = strings.TrimSpace(raw); raw == "" {
			continue
		}
		id, err := strconv.ParseInt(raw, 10, 64)
		if err != nil || id < 0 {
			return stats.AttendanceQuery{}, apperror.BadRequest("invalid churches")
		}
		churches = append(churches, id)
	}
	return stats.AttendanceQuery{Year: year, Metric: c.QueryParam("metric"), Status: c.QueryParam("status"), Period: period, Compare: compare, ChurchIDs: churches}, nil
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
