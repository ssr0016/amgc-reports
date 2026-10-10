package apperror

import (
	"errors"
	"fmt"
	"net/http"
	"testing"

	"github.com/labstack/echo/v4"
)

func TestStatusOf(t *testing.T) {
	tests := []struct {
		name string
		err  error
		want int
	}{
		{"unauthorized", Unauthorized("x"), http.StatusUnauthorized},
		{"forbidden", Forbidden("x"), http.StatusForbidden},
		{"validation", Validation("x"), http.StatusUnprocessableEntity},
		{"rate limit", RateLimit("x"), http.StatusTooManyRequests},
		{"conflict", Conflict("x"), http.StatusConflict},
		{"wrapped app error", fmt.Errorf("wrap: %w", NotFound("x")), http.StatusNotFound},
		{"echo http error", echo.NewHTTPError(http.StatusNotFound, "nope"), http.StatusNotFound},
		{"wrapped echo http error", fmt.Errorf("wrap: %w", echo.NewHTTPError(http.StatusMethodNotAllowed)), http.StatusMethodNotAllowed},
		{"unknown error", errors.New("boom"), http.StatusInternalServerError},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := StatusOf(tt.err); got != tt.want {
				t.Errorf("StatusOf() = %d, want %d", got, tt.want)
			}
		})
	}
}
