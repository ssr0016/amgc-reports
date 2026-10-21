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
// @Description  Counts for one month: reports (total, approved, pending) and, regardless of month, all users and admins. Admin role only. Without month and year the period is the previous month in Asia/Manila. Month and year go together; the month is an English month name, case-insensitive.
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
	var month *string
	var year *int
	if m := c.QueryParam("month"); m != "" {
		month = &m
	}
	if y := c.QueryParam("year"); y != "" {
		n, err := strconv.Atoi(y)
		if err != nil || n <= 0 {
			return apperror.BadRequest("invalid year")
		}
		year = &n
	}
	out, err := h.svc.Get(c.Request().Context(), stats.GetStatsQuery{Month: month, Year: year})
	switch {
	case errors.Is(err, stats.ErrPeriod):
		msg := stats.ErrPeriod.Error()
		return apperror.Validation(msg).WithDetails(map[string]string{"month": msg})
	case err != nil:
		return apperror.Internal("failed to get stats").WithError(err)
	}
	return c.JSON(http.StatusOK, out)
}
