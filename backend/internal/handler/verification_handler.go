package handler

import (
	"net/http"

	"github.com/labstack/echo/v4"

	"github.com/ssr0016/template/internal/apperror"
	"github.com/ssr0016/template/internal/service"
)

type VerificationHandler struct {
	service *service.VerificationService
}

func NewVerificationHandler(svc *service.VerificationService) *VerificationHandler {
	return &VerificationHandler{service: svc}
}

// VerifyEmail godoc
// @Summary      Verify email with token
// @Tags         auth
// @Produce      json
// @Param        token query string true "Verification token"
// @Success      200 {object} map[string]string
// @Failure      400 {object} map[string]string
// @Router       /auth/verify-email [get]
func (h *VerificationHandler) VerifyEmail(c echo.Context) error {
	token := c.QueryParam("token")
	if token == "" {
		return apperror.BadRequest("token is required")
	}

	if err := h.service.Verify(c.Request().Context(), token); err != nil {
		if err == service.ErrInvalidToken {
			return apperror.BadRequest("invalid or expired token")
		}
		return apperror.Internal("verification failed").WithError(err)
	}

	return c.JSON(http.StatusOK, map[string]string{
		"message": "email verified successfully",
	})
}
