package rest

import (
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"strings"

	"github.com/labstack/echo/v4"

	"github.com/ssr0016/pastoral-reports/backend/internal/apperror"
	"github.com/ssr0016/pastoral-reports/backend/internal/database"
	"github.com/ssr0016/pastoral-reports/backend/internal/report"
	"github.com/ssr0016/pastoral-reports/backend/internal/request"
	"github.com/ssr0016/pastoral-reports/backend/pkg/pagination"
)

type ReportHandler struct {
	svc report.Service
}

func NewReportHandler(svc report.Service) *ReportHandler {
	return &ReportHandler{svc: svc}
}

// reportError maps service errors to API errors. A session user that no longer
// exists is a 401; ErrReportNotFound covers missing, deleted and not-yours (D-1).
func reportError(err error, msg string) error {
	switch {
	case errors.Is(err, report.ErrReportNotFound):
		return apperror.NotFound("report not found")
	case errors.Is(err, report.ErrReportExists):
		return apperror.Conflict(report.ErrReportExists.Error())
	case errors.Is(err, report.ErrTooManyReports):
		return apperror.Validation("A bulk export holds at most 100 approved reports.")
	case errors.Is(err, report.ErrPeriodNotAllowed):
		msg := "Reports can only be created for the previous month."
		return apperror.Validation(msg).WithDetails(map[string]string{"month": msg})
	case errors.Is(err, database.ErrNotFound):
		return apperror.Unauthorized("user not found")
	}
	return apperror.Internal(msg).WithError(err)
}

func (h *ReportHandler) userID(c echo.Context) (int64, error) {
	u, ok := request.UserFrom(c.Request().Context())
	if !ok {
		return 0, apperror.Unauthorized("authentication required")
	}
	return u.ID, nil
}

func reportID(c echo.Context) (int64, error) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil || id <= 0 {
		return 0, apperror.BadRequest("invalid report id")
	}
	return id, nil
}

// CreateReport godoc
// @Summary      Create report
// @Description  Files a monthly report for the current user. A missing year defaults to the current year.
// @Tags         reports
// @Accept       json
// @Produce      json
// @Security     CookieAuth
// @Param        body body report.CreateReportRequest true "Report payload"
// @Success      201 {object} report.ReportResponse
// @Failure      400 {object} map[string]string
// @Failure      401 {object} map[string]string
// @Failure      403 {object} map[string]string
// @Failure      409 {object} map[string]string
// @Failure      422 {object} map[string]string
// @Router       /reports [post]
func (h *ReportHandler) CreateReport(c echo.Context) error {
	uid, err := h.userID(c)
	if err != nil {
		return err
	}
	var req report.CreateReportRequest
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

// GetReport godoc
// @Summary      Get report
// @Description  Returns one report. Without reports:read_any only the caller's own report is visible; another user's report is 404.
// @Tags         reports
// @Produce      json
// @Security     CookieAuth
// @Param        id path int true "Report ID"
// @Success      200 {object} report.ReportResponse
// @Failure      400 {object} map[string]string
// @Failure      401 {object} map[string]string
// @Failure      403 {object} map[string]string
// @Failure      404 {object} map[string]string
// @Router       /reports/{id} [get]
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

// ListReports godoc
// @Summary      List reports
// @Description  Returns a page of reports, newest first. Without reports:read_any the list is limited to the caller's own reports.
// @Tags         reports
// @Produce      json
// @Security     CookieAuth
// @Param        page query int false "Page number (default 1)"
// @Param        limit query int false "Limit (default 20, max 100)"
// @Param        month query string false "Month name, case-insensitive"
// @Param        year query int false "Year"
// @Param        worker query string false "Partial, case-insensitive match"
// @Param        area query string false "Partial, case-insensitive match on area assignment"
// @Param        church query string false "Partial, case-insensitive match on church name"
// @Success      200 {object} pagination.Response[report.ReportResponse]
// @Failure      400 {object} map[string]string
// @Failure      401 {object} map[string]string
// @Failure      403 {object} map[string]string
// @Router       /reports [get]
func (h *ReportHandler) ListReports(c echo.Context) error {
	uid, err := h.userID(c)
	if err != nil {
		return err
	}
	p := report.ListReportsParams{
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
	pg := pagination.FromQuery(c.QueryParams())
	reports, total, err := h.svc.List(c.Request().Context(), uid, p, pg)
	if err != nil {
		return reportError(err, "failed to list reports")
	}
	if reports == nil {
		reports = []report.ReportResponse{} // JSON [] not null
	}
	return c.JSON(http.StatusOK, pagination.Response[report.ReportResponse]{
		Data: reports,
		Meta: pagination.NewMeta(pg, total),
	})
}

// UpdateReport godoc
// @Summary      Update report
// @Description  Replaces the report's fields and its whole metric set. A missing year keeps the current year. Without reports:write_any only the caller's own report can be updated.
// @Tags         reports
// @Accept       json
// @Produce      json
// @Security     CookieAuth
// @Param        id path int true "Report ID"
// @Param        body body report.UpdateReportRequest true "Report payload"
// @Success      200 {object} report.ReportResponse
// @Failure      400 {object} map[string]string
// @Failure      401 {object} map[string]string
// @Failure      403 {object} map[string]string
// @Failure      404 {object} map[string]string
// @Failure      409 {object} map[string]string
// @Failure      422 {object} map[string]string
// @Router       /reports/{id} [put]
func (h *ReportHandler) UpdateReport(c echo.Context) error {
	uid, err := h.userID(c)
	if err != nil {
		return err
	}
	id, err := reportID(c)
	if err != nil {
		return err
	}
	var req report.UpdateReportRequest
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

// SoftDeleteReport godoc
// @Summary      Delete report
// @Description  Soft-deletes a report. Without reports:delete_any only the caller's own report can be deleted.
// @Tags         reports
// @Security     CookieAuth
// @Param        id path int true "Report ID"
// @Success      204
// @Failure      400 {object} map[string]string
// @Failure      401 {object} map[string]string
// @Failure      403 {object} map[string]string
// @Failure      404 {object} map[string]string
// @Router       /reports/{id} [delete]
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

// ToggleComplete godoc
// @Summary      Approve or unapprove report
// @Description  Flips the report's completed flag. Requires reports:approve, which works on any report by ID. Returns metadata only, never report text.
// @Tags         reports
// @Produce      json
// @Security     CookieAuth
// @Param        id path int true "Report ID"
// @Success      200 {object} report.ReportMetadata
// @Failure      400 {object} map[string]string
// @Failure      401 {object} map[string]string
// @Failure      403 {object} map[string]string
// @Failure      404 {object} map[string]string
// @Router       /reports/{id}/complete [patch]
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

const xlsxMIME = "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet"

// sendExport writes the workbook as an attachment. filename* carries the UTF-8
// name (RFC 5987); filename is an ASCII fallback for older clients.
func sendExport(c echo.Context, x *report.Export) error {
	fallback := strings.Map(func(r rune) rune {
		if r > 0x7e || r == '"' || r == ';' {
			return '_'
		}
		return r
	}, x.Filename)
	h := c.Response().Header()
	h.Set(echo.HeaderContentDisposition, fmt.Sprintf(`attachment; filename="%s"; filename*=UTF-8''%s`, fallback, url.PathEscape(x.Filename)))
	h.Set("Cache-Control", "no-store")
	return c.Blob(http.StatusOK, xlsxMIME, x.Data)
}

// ExportReport godoc
// @Summary      Download one report as Excel
// @Description  Admin only. Returns the report in the legacy Church Growth Program Report layout (D-A6) and writes a DOWNLOAD_SINGLE_EXCEL audit row.
// @Tags         admin
// @Produce      application/vnd.openxmlformats-officedocument.spreadsheetml.sheet
// @Security     CookieAuth
// @Param        id path int true "Report ID"
// @Success      200 {file} file
// @Failure      400 {object} map[string]string
// @Failure      401 {object} map[string]string
// @Failure      403 {object} map[string]string
// @Failure      404 {object} map[string]string
// @Router       /admin/reports/{id}/excel [post]
func (h *ReportHandler) ExportReport(c echo.Context) error {
	uid, err := h.userID(c)
	if err != nil {
		return err
	}
	id, err := reportID(c)
	if err != nil {
		return err
	}
	x, err := h.svc.ExportOne(c.Request().Context(), uid, c.RealIP(), id)
	if err != nil {
		return reportError(err, "failed to export report")
	}
	return sendExport(c, x)
}

// ExportReports godoc
// @Summary      Download a month of reports as Excel
// @Description  Admin only. A Report Dashboard sheet (title, then Churches, Status, Coordinator's Remark, Support Status) with every church account's status for the month (Approved, Pending or Not Filed), then one sheet per approved report (at most 100) in the legacy layout and styling (D-A6, D-A16, D-A17). A month with no approved reports gives the dashboard alone. Writes a DOWNLOAD_BULK_EXCEL audit row whose count is the number of report sheets.
// @Tags         admin
// @Accept       json
// @Produce      application/vnd.openxmlformats-officedocument.spreadsheetml.sheet
// @Security     CookieAuth
// @Param        body body report.ExportReportsRequest true "Month and year"
// @Success      200 {file} file
// @Failure      400 {object} map[string]string
// @Failure      401 {object} map[string]string
// @Failure      403 {object} map[string]string
// @Failure      422 {object} map[string]string
// @Router       /admin/reports/excel [post]
func (h *ReportHandler) ExportReports(c echo.Context) error {
	uid, err := h.userID(c)
	if err != nil {
		return err
	}
	var req report.ExportReportsRequest
	if err := c.Bind(&req); err != nil {
		return apperror.BadRequest("invalid request body").WithError(err)
	}
	if err := c.Validate(&req); err != nil {
		return err
	}
	x, err := h.svc.ExportMonth(c.Request().Context(), uid, c.RealIP(), req)
	if err != nil {
		return reportError(err, "failed to export reports")
	}
	return sendExport(c, x)
}

// PreviewExcel godoc
// @Summary      Preview an Excel export
// @Description  Admin only. Builds the workbook the export would build, for one report (report_id) or one month (month and year), and returns each sheet's name and cell text: A1:G42 for a report sheet, title, header and one row per account for the month's Report Dashboard. count is the number of report sheets. Writes no audit row (D-A15, D-A16, D-A17).
// @Tags         admin
// @Accept       json
// @Produce      json
// @Security     CookieAuth
// @Param        body body report.ExcelPreviewRequest true "One report, or one month"
// @Success      200 {object} report.ExcelPreview
// @Failure      400 {object} map[string]string
// @Failure      401 {object} map[string]string
// @Failure      403 {object} map[string]string
// @Failure      404 {object} map[string]string
// @Failure      422 {object} map[string]string
// @Router       /admin/reports/excel/preview [post]
func (h *ReportHandler) PreviewExcel(c echo.Context) error {
	uid, err := h.userID(c)
	if err != nil {
		return err
	}
	var req report.ExcelPreviewRequest
	if err := c.Bind(&req); err != nil {
		return apperror.BadRequest("invalid request body").WithError(err)
	}
	if err := c.Validate(&req); err != nil {
		return err
	}
	p, err := h.svc.Preview(c.Request().Context(), uid, req)
	if err != nil {
		return reportError(err, "failed to preview export")
	}
	// The preview holds free text and tithes, like the file.
	c.Response().Header().Set("Cache-Control", "no-store")
	return c.JSON(http.StatusOK, p)
}
