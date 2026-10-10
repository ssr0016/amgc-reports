package handler

import (
	"net/http"

	"github.com/labstack/echo/v4"

	"github.com/ssr0016/template/internal/apperror"
	"github.com/ssr0016/template/internal/service"
)

type PasswordResetHandler struct {
	service *service.PasswordResetService
}

func NewPasswordResetHandler(svc *service.PasswordResetService) *PasswordResetHandler {
	return &PasswordResetHandler{service: svc}
}

type ForgotPasswordRequest struct {
	Email string `json:"email" validate:"required,email"`
}

type ResetPasswordRequest struct {
	Token       string `json:"token" validate:"required"`
	NewPassword string `json:"new_password" validate:"required,min=8,max=72"`
}

// ForgotPassword godoc
// @Summary      Request password reset
// @Description  Sends a password reset token to the email if it exists
// @Tags         auth
// @Accept       json
// @Produce      json
// @Param        body body ForgotPasswordRequest true "Email payload"
// @Success      200 {object} map[string]string
// @Failure      400 {object} map[string]string
// @Router       /auth/forgot-password [post]
func (h *PasswordResetHandler) ForgotPassword(c echo.Context) error {
	var req ForgotPasswordRequest
	if err := c.Bind(&req); err != nil {
		return apperror.BadRequest("invalid request body").WithError(err)
	}
	if err := c.Validate(&req); err != nil {
		return apperror.Validation(err.Error())
	}

	if err := h.service.RequestReset(c.Request().Context(), req.Email); err != nil {
		return apperror.Internal("failed to process request").WithError(err)
	}

	return c.JSON(http.StatusOK, map[string]string{
		"message": "if the email exists, a reset link has been sent",
	})
}

// ResetPassword godoc
// @Summary      Reset password with token
// @Description  Resets the user's password using a valid reset token
// @Tags         auth
// @Accept       json
// @Produce      json
// @Param        body body ResetPasswordRequest true "Reset payload"
// @Success      200 {object} map[string]string
// @Failure      400 {object} map[string]string
// @Router       /auth/reset-password [post]
func (h *PasswordResetHandler) ResetPassword(c echo.Context) error {
	var req ResetPasswordRequest
	if err := c.Bind(&req); err != nil {
		return apperror.BadRequest("invalid request body").WithError(err)
	}
	if err := c.Validate(&req); err != nil {
		return apperror.Validation(err.Error())
	}

	if err := h.service.ResetPassword(c.Request().Context(), req.Token, req.NewPassword); err != nil {
		if err == service.ErrInvalidResetToken {
			return apperror.BadRequest("invalid or expired token")
		}
		return apperror.Internal("password reset failed").WithError(err)
	}

	return c.JSON(http.StatusOK, map[string]string{
		"message": "password reset successful",
	})
}
