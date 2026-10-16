package handler

import (
	"context"
	"errors"
	"net/http"
	"strconv"

	"github.com/labstack/echo/v4"

	"github.com/ssr0016/pastoral-reports/backend/internal/apperror"
	"github.com/ssr0016/pastoral-reports/backend/internal/model"
	"github.com/ssr0016/pastoral-reports/backend/internal/service"
)

// StatsServiceInterface is what StatsHandler needs from the stats service, so
// tests can pass a fake. main passes *service.StatsService.
type StatsServiceInterface interface {
	Get(ctx context.Context, month *string, year *int) (model.AdminStats, error)
}

var _ StatsServiceInterface = (*service.StatsService)(nil)

type StatsHandler struct {
	svc StatsServiceInterface
}

func NewStatsHandler(svc StatsServiceInterface) *StatsHandler { return &StatsHandler{svc: svc} }

// GetStats godoc
// @Summary      Admin panel counts
// @Description  Counts for one month: reports (total, approved, pending) and, regardless of month, all users and admins. Admin role only. Without month and year the period is the previous month in Asia/Manila. Month and year go together; the month is an English month name, case-insensitive.
// @Tags         admin/stats
// @Produce      json
// @Security     CookieAuth
// @Param        month query string false "Month name, case-insensitive; requires year"
// @Param        year query int false "Year; requires month"
// @Success      200 {object} model.AdminStats
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
	stats, err := h.svc.Get(c.Request().Context(), month, year)
	switch {
	case errors.Is(err, service.ErrStatsPeriod):
		msg := service.ErrStatsPeriod.Error()
		return apperror.Validation(msg).WithDetails(map[string]string{"month": msg})
	case err != nil:
		return apperror.Internal("failed to get stats").WithError(err)
	}
	return c.JSON(http.StatusOK, stats)
}
