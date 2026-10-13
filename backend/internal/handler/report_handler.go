package handler

import (
	"context"
	"errors"
	"net/http"
	"strconv"

	"github.com/alexedwards/scs/v2"
	"github.com/labstack/echo/v4"

	"github.com/ssr0016/pastoral-reports/backend/internal/apperror"
	"github.com/ssr0016/pastoral-reports/backend/internal/model"
	"github.com/ssr0016/pastoral-reports/backend/internal/repository"
	"github.com/ssr0016/pastoral-reports/backend/internal/service"
	"github.com/ssr0016/pastoral-reports/backend/pkg/pagination"
)

// ReportServiceInterface is what ReportHandler needs from the report service,
// so tests can pass a fake. main passes *service.ReportService.
type ReportServiceInterface interface {
	Create(ctx context.Context, userID int64, ip string, req model.CreateReportRequest) (*model.ReportResponse, error)
	Get(ctx context.Context, userID, id int64) (*model.ReportResponse, error)
	List(ctx context.Context, userID int64, p model.ListReportsParams, pg pagination.Params) ([]model.ReportResponse, int64, error)
	Update(ctx context.Context, userID int64, ip string, id int64, req model.UpdateReportRequest) (*model.ReportResponse, error)
	Delete(ctx context.Context, userID int64, ip string, id int64) error
	ToggleComplete(ctx context.Context, userID int64, ip string, id int64) (*model.ReportMetadata, error)
}

var _ ReportServiceInterface = (*service.ReportService)(nil)

type ReportHandler struct {
	svc ReportServiceInterface
	sm  *scs.SessionManager
}

func NewReportHandler(svc ReportServiceInterface, sm *scs.SessionManager) *ReportHandler {
	return &ReportHandler{svc: svc, sm: sm}
}

// reportError maps service errors to API errors. A session user that no longer
// exists is a 401; ErrReportNotFound covers missing, deleted and not-yours (D-1).
func reportError(err error, msg string) error {
	switch {
	case errors.Is(err, service.ErrReportNotFound):
		return apperror.NotFound("report not found")
	case errors.Is(err, service.ErrReportExists):
		return apperror.Conflict(service.ErrReportExists.Error())
	case errors.Is(err, repository.ErrNotFound):
		return apperror.Unauthorized("user not found")
	}
	return apperror.Internal(msg).WithError(err)
}

func (h *ReportHandler) userID(c echo.Context) (int64, error) {
	id := h.sm.GetInt64(c.Request().Context(), "user_id")
	if id == 0 {
		return 0, apperror.Unauthorized("authentication required")
	}
	return id, nil
}

func reportID(c echo.Context) (int64, error) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil || id <= 0 {
		return 0, apperror.BadRequest("invalid report id")
	}
	return id, nil
}

func (h *ReportHandler) CreateReport(c echo.Context) error {
	uid, err := h.userID(c)
	if err != nil {
		return err
	}
	var req model.CreateReportRequest
	if err := c.Bind(&req); err != nil {
		return apperror.BadRequest("invalid request body").WithError(err)
	}
	if err := c.Validate(&req); err != nil {
		return err
	}
	rep, err := h.svc.Create(c.Request().Context(), uid, c.RealIP(), req)
	if err != nil {
		return reportError(err, "failed to create report")
	}
	return c.JSON(http.StatusCreated, rep)
}

func (h *ReportHandler) GetReport(c echo.Context) error {
	uid, err := h.userID(c)
	if err != nil {
		return err
	}
	id, err := reportID(c)
	if err != nil {
		return err
	}
	rep, err := h.svc.Get(c.Request().Context(), uid, id)
	if err != nil {
		return reportError(err, "failed to get report")
	}
	return c.JSON(http.StatusOK, rep)
}

func (h *ReportHandler) ListReports(c echo.Context) error {
	uid, err := h.userID(c)
	if err != nil {
		return err
	}
	p := model.ListReportsParams{
		Month:  c.QueryParam("month"),
		Worker: c.QueryParam("worker"),
		Area:   c.QueryParam("area"),
		Church: c.QueryParam("church"),
	}
	if y := c.QueryParam("year"); y != "" {
		if p.Year, err = strconv.Atoi(y); err != nil || p.Year <= 0 {
			return apperror.BadRequest("invalid year")
		}
	}
	pg := pagination.FromContext(c)
	reports, total, err := h.svc.List(c.Request().Context(), uid, p, pg)
	if err != nil {
		return reportError(err, "failed to list reports")
	}
	if reports == nil {
		reports = []model.ReportResponse{} // JSON [] not null
	}
	return c.JSON(http.StatusOK, pagination.Response[model.ReportResponse]{
		Data: reports,
		Meta: pagination.NewMeta(pg, total),
	})
}

func (h *ReportHandler) UpdateReport(c echo.Context) error {
	uid, err := h.userID(c)
	if err != nil {
		return err
	}
	id, err := reportID(c)
	if err != nil {
		return err
	}
	var req model.UpdateReportRequest
	if err := c.Bind(&req); err != nil {
		return apperror.BadRequest("invalid request body").WithError(err)
	}
	if err := c.Validate(&req); err != nil {
		return err
	}
	rep, err := h.svc.Update(c.Request().Context(), uid, c.RealIP(), id, req)
	if err != nil {
		return reportError(err, "failed to update report")
	}
	return c.JSON(http.StatusOK, rep)
}

func (h *ReportHandler) SoftDeleteReport(c echo.Context) error {
	uid, err := h.userID(c)
	if err != nil {
		return err
	}
	id, err := reportID(c)
	if err != nil {
		return err
	}
	if err := h.svc.Delete(c.Request().Context(), uid, c.RealIP(), id); err != nil {
		return reportError(err, "failed to delete report")
	}
	return c.NoContent(http.StatusNoContent)
}

// ToggleComplete returns metadata only, never report text (D-1).
func (h *ReportHandler) ToggleComplete(c echo.Context) error {
	uid, err := h.userID(c)
	if err != nil {
		return err
	}
	id, err := reportID(c)
	if err != nil {
		return err
	}
	meta, err := h.svc.ToggleComplete(c.Request().Context(), uid, c.RealIP(), id)
	if err != nil {
		return reportError(err, "failed to update report status")
	}
	return c.JSON(http.StatusOK, meta)
}
