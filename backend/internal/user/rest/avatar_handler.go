package rest

import (
	"bytes"
	"context"
	"crypto/rand"
	"errors"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/labstack/echo/v4"

	"github.com/ssr0016/pastoral-reports/backend/internal/apperror"
	"github.com/ssr0016/pastoral-reports/backend/internal/database"
	"github.com/ssr0016/pastoral-reports/backend/internal/logger"
	"github.com/ssr0016/pastoral-reports/backend/internal/middleware"
	"github.com/ssr0016/pastoral-reports/backend/internal/request"
	"github.com/ssr0016/pastoral-reports/backend/internal/user"
)

// AvatarStorage stores public avatars and deletes them by their public URLs.
type AvatarStorage interface {
	Put(ctx context.Context, key, contentType string, data []byte) (string, error)
	Delete(ctx context.Context, url string) error
}

type AvatarHandler struct {
	repo    user.Store
	storage AvatarStorage // nil when R2 is not configured
}

func NewAvatarHandler(repo user.Store, storage AvatarStorage) *AvatarHandler {
	return &AvatarHandler{repo: repo, storage: storage}
}

// UploadAvatar godoc
// @Summary      Upload my profile picture
// @Description  Replaces the signed-in user's avatar. JPEG, PNG or WebP, at most 512 pixels per side; the whole request is capped at 1 MiB (413 above it), so the client should downscale first.
// @Tags         users
// @Accept       multipart/form-data
// @Produce      json
// @Security     CookieAuth
// @Param        avatar formData file true "Image file"
// @Success      200 {object} user.UserResponse
// @Failure      401 {object} map[string]string
// @Failure      413 {object} map[string]string
// @Failure      422 {object} map[string]string
// @Failure      503 {object} map[string]string
// @Router       /users/me/avatar [post]
func (h *AvatarHandler) UploadAvatar(c echo.Context) error {
	if h.storage == nil {
		return apperror.New(apperror.CodeInternal, "avatar uploads are not configured", http.StatusServiceUnavailable)
	}
	ctx := c.Request().Context()
	caller, ok := request.UserFrom(ctx)
	if !ok {
		return apperror.Unauthorized("not authenticated")
	}

	data, err := readAvatar(c)
	if err != nil {
		return err
	}
	contentType, err := validateAvatarImage(data)
	if err != nil {
		return err
	}
	u, err := h.saveAvatar(ctx, caller.ID, contentType, data)
	if err != nil {
		return err
	}
	return c.JSON(http.StatusOK, u.ToResponse())
}

func readAvatar(c echo.Context) ([]byte, error) {
	req := c.Request()
	body, err := io.ReadAll(http.MaxBytesReader(c.Response(), req.Body, middleware.MaxRequestBodyBytes))
	if err != nil {
		var limitErr *http.MaxBytesError
		if errors.As(err, &limitErr) || errors.Is(err, echo.ErrStatusRequestEntityTooLarge) {
			return nil, echo.ErrStatusRequestEntityTooLarge
		}
		return nil, apperror.BadRequest("invalid upload").WithError(err)
	}
	if err := req.Body.Close(); err != nil {
		return nil, apperror.BadRequest("invalid upload").WithError(err)
	}
	req.Body = io.NopCloser(bytes.NewReader(body))
	fh, err := c.FormFile("avatar")
	if err != nil {
		return nil, fieldError("avatar", "is required")
	}
	f, err := fh.Open()
	if err != nil {
		return nil, apperror.BadRequest("invalid file").WithError(err)
	}
	defer func() { _ = f.Close() }()
	data, err := io.ReadAll(f)
	if err != nil {
		return nil, apperror.BadRequest("invalid file").WithError(err)
	}
	return data, nil
}

func (h *AvatarHandler) saveAvatar(ctx context.Context, userID int64, contentType string, data []byte) (*user.User, error) {
	url, err := h.storage.Put(ctx, fmt.Sprintf("avatars/%d/%s", userID, rand.Text()), contentType, data)
	if err != nil {
		return nil, apperror.Internal("failed to store avatar").WithError(err)
	}
	u, previousURL, err := h.repo.UpdateAvatar(ctx, userID, url)
	if err != nil {
		// A lost commit response can mean the URL was saved. Confirm before deleting it.
		lookupCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 5*time.Second)
		current, lookupErr := h.repo.GetByID(lookupCtx, userID)
		cancel()
		if lookupErr == nil && (current == nil || current.AvatarURL == nil || *current.AvatarURL != url) {
			h.deleteAvatar(ctx, url)
		} else if lookupErr != nil {
			logger.FromContext(ctx).Error("could not confirm failed avatar update; retaining uploaded object", "url", url, "error", lookupErr)
		}
		if errors.Is(err, database.ErrNotFound) {
			return nil, apperror.NotFound("user not found")
		}
		return nil, apperror.Internal("failed to save avatar").WithError(err)
	}
	if previousURL != "" {
		h.deleteAvatar(ctx, previousURL)
	}
	return u, nil
}

func (h *AvatarHandler) deleteAvatar(ctx context.Context, url string) {
	cleanupCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 5*time.Second)
	defer cancel()
	if err := h.storage.Delete(cleanupCtx, url); err != nil {
		logger.FromContext(ctx).Error("failed to delete unused avatar", "url", url, "error", err)
	}
}
