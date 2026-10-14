package handler

import (
	"errors"
	"net/http"

	"github.com/alexedwards/scs/v2"
	"github.com/labstack/echo/v4"

	"github.com/ssr0016/pastoral-reports/backend/internal/apperror"
	"github.com/ssr0016/pastoral-reports/backend/internal/model"
	"github.com/ssr0016/pastoral-reports/backend/internal/service"
)

type AuthHandler struct {
	authService *service.AuthService
	sm          *scs.SessionManager
}

func NewAuthHandler(authService *service.AuthService, sm *scs.SessionManager) *AuthHandler {
	return &AuthHandler{authService: authService, sm: sm}
}

// Login godoc
// @Summary      Login user
// @Description  Authenticates user and creates a session
// @Tags         auth
// @Accept       json
// @Produce      json
// @Param        body body model.LoginRequest true "Login payload"
// @Success      200 {object} model.UserResponse
// @Failure      401 {object} map[string]string
// @Failure      403 {object} map[string]string
// @Router       /auth/login [post]
func (h *AuthHandler) Login(c echo.Context) error {
	var req model.LoginRequest
	if err := c.Bind(&req); err != nil {
		return apperror.BadRequest("invalid request body").WithError(err)
	}
	if err := c.Validate(&req); err != nil {
		return err
	}

	user, err := h.authService.Login(c.Request().Context(), req.Email, req.Password)
	if err != nil {
		if errors.Is(err, service.ErrInvalidCredentials) {
			return apperror.Unauthorized("invalid email or password")
		}
		if errors.Is(err, service.ErrAccountLocked) {
			return apperror.Forbidden("account is locked, try again later")
		}
		return apperror.Internal("login failed").WithError(err)
	}

	ctx := c.Request().Context()
	if err := h.sm.RenewToken(ctx); err != nil {
		return apperror.Internal("session error").WithError(err)
	}
	h.sm.Put(ctx, "user_id", user.ID)

	return c.JSON(http.StatusOK, user.ToResponse())
}

// Logout godoc
// @Summary      Logout user
// @Description  Destroys the current session
// @Tags         auth
// @Produce      json
// @Security     CookieAuth
// @Success      200 {object} map[string]string
// @Router       /auth/logout [post]
func (h *AuthHandler) Logout(c echo.Context) error {
	if err := h.sm.Destroy(c.Request().Context()); err != nil {
		return apperror.Internal("logout failed").WithError(err)
	}
	return c.JSON(http.StatusOK, map[string]string{"message": "logged out"})
}

// Me godoc
// @Summary      Get current user
// @Description  Returns the authenticated user's ID, name, email, role and permissions, read live from the database. Use it to show or hide actions; the server still enforces every permission.
// @Tags         auth
// @Produce      json
// @Security     CookieAuth
// @Success      200 {object} model.MeResponse
// @Failure      401 {object} map[string]string
// @Router       /auth/me [get]
func (h *AuthHandler) Me(c echo.Context) error {
	userID := h.sm.GetInt64(c.Request().Context(), "user_id")
	if userID == 0 {
		return apperror.Unauthorized("not authenticated")
	}
	me, err := h.authService.Me(c.Request().Context(), userID)
	if err != nil {
		if errors.Is(err, service.ErrUnauthorized) {
			return apperror.Unauthorized("not authenticated")
		}
		return apperror.Internal("failed to load user").WithError(err)
	}
	return c.JSON(http.StatusOK, me)
}
