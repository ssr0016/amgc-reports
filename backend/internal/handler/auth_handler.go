package handler

import (
	"errors"
	"net/http"

	"github.com/alexedwards/scs/v2"
	"github.com/labstack/echo/v4"

	"github.com/ssr0016/template/internal/apperror"
	"github.com/ssr0016/template/internal/model"
	"github.com/ssr0016/template/internal/service"
)

type AuthHandler struct {
	authService *service.AuthService
	sm          *scs.SessionManager
}

func NewAuthHandler(authService *service.AuthService, sm *scs.SessionManager) *AuthHandler {
	return &AuthHandler{authService: authService, sm: sm}
}

// Register godoc
// @Summary      Register new user
// @Description  Creates a new user account
// @Tags         auth
// @Accept       json
// @Produce      json
// @Param        body body model.RegisterRequest true "Registration payload"
// @Success      201 {object} model.UserResponse
// @Failure      400 {object} map[string]string
// @Failure      409 {object} map[string]string
// @Failure      422 {object} map[string]string
// @Router       /auth/register [post]
func (h *AuthHandler) Register(c echo.Context) error {
	var req model.RegisterRequest
	if err := c.Bind(&req); err != nil {
		return apperror.BadRequest("invalid request body").WithError(err)
	}
	if err := c.Validate(&req); err != nil {
		return apperror.Validation(err.Error())
	}

	user, err := h.authService.Register(c.Request().Context(), req)
	if err != nil {
		if errors.Is(err, service.ErrEmailTaken) {
			return apperror.Conflict(err.Error())
		}
		return apperror.Internal("registration failed").WithError(err)
	}
	return c.JSON(http.StatusCreated, user.ToResponse())
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
		return apperror.Validation(err.Error())
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
// @Description  Returns the authenticated user's ID
// @Tags         auth
// @Produce      json
// @Security     CookieAuth
// @Success      200 {object} map[string]int64
// @Failure      401 {object} map[string]string
// @Router       /auth/me [get]
func (h *AuthHandler) Me(c echo.Context) error {
	userID := h.sm.GetInt64(c.Request().Context(), "user_id")
	if userID == 0 {
		return apperror.Unauthorized("not authenticated")
	}
	return c.JSON(http.StatusOK, map[string]int64{"user_id": userID})
}
