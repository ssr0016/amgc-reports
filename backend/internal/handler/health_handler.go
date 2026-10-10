package handler

import (
	"context"
	"net/http"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/labstack/echo/v4"
)

type HealthHandler struct {
	pool *pgxpool.Pool
}

func NewHealthHandler(pool *pgxpool.Pool) *HealthHandler {
	return &HealthHandler{pool: pool}
}

// HealthCheck godoc
// @Summary      Health check
// @Description  Returns OK if the service is running
// @Tags         system
// @Produce      json
// @Success      200 {object} map[string]string
// @Router       /health [get]
func (h *HealthHandler) HealthCheck(c echo.Context) error {
	return c.JSON(http.StatusOK, map[string]string{"status": "ok"})
}

// Readiness godoc
// @Summary      Readiness check
// @Description  Returns OK if the DB is connected
// @Tags         system
// @Produce      json
// @Success      200 {object} map[string]string
// @Failure      503 {object} map[string]string
// @Router       /ready [get]
func (h *HealthHandler) Readiness(c echo.Context) error {
	ctx, cancel := context.WithTimeout(c.Request().Context(), 3*time.Second)
	defer cancel()

	if err := h.pool.Ping(ctx); err != nil {
		return c.JSON(http.StatusServiceUnavailable, map[string]string{
			"status": "not ready",
			"error":  err.Error(),
		})
	}
	return c.JSON(http.StatusOK, map[string]string{
		"status": "ready",
		"db":     "connected",
	})
}

// Liveness godoc
// @Summary      Liveness check
// @Description  Returns OK if the process is alive
// @Tags         system
// @Produce      json
// @Success      200 {object} map[string]string
// @Router       /live [get]
func (h *HealthHandler) Liveness(c echo.Context) error {
	return c.JSON(http.StatusOK, map[string]string{"status": "alive"})
}
