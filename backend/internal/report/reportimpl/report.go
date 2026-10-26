package reportimpl

import (
	"cmp"
	"context"
	"errors"
	"fmt"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/xuri/excelize/v2"

	"github.com/ssr0016/pastoral-reports/backend/internal/audit"
	"github.com/ssr0016/pastoral-reports/backend/internal/database"
	"github.com/ssr0016/pastoral-reports/backend/internal/report"
	"github.com/ssr0016/pastoral-reports/backend/internal/user"
	"github.com/ssr0016/pastoral-reports/backend/pkg/pagination"
)

// service holds the report business rules: year default, ownership scope
// (D-1), and one transaction per mutation covering report, metrics and audit
// (D-10, D-14). Route-level RequirePermission decides who may call a method at
// all; this service only applies the _any scope.
//
// Note the store asymmetry it hides: report.Store.Get returns
// (nil, nil) for a missing report, while Update, SoftDelete and ToggleComplete
// return ErrNotFound. Both become report.ErrReportNotFound here.
type service struct {
	reports report.Store
	audit   audit.Store
	tx      database.TxRunner
	users   user.Store
	now     func() time.Time
	loc     *time.Location // months are counted in Philippine time, whatever the server clock says
}

// NewService returns the report.Service over its stores.
func NewService(
	reports report.Store,
	audit audit.Store,
	tx database.TxRunner,
	users user.Store,
) report.Service {
	return newService(reports, audit, tx, users)
}

// newService is NewService with the concrete type, so tests can set the clock.
func newService(
	reports report.Store,
	audit audit.Store,
	tx database.TxRunner,
	users user.Store,
) *service {
	loc, err := time.LoadLocation("Asia/Manila")
	if err != nil {
		panic(fmt.Sprintf("load Asia/Manila (is tzdata installed?): %v", err))
	}
	return &service{reports: reports, audit: audit, tx: tx, users: users, now: time.Now, loc: loc}
}

type actor struct {
	user  *user.User
	perms map[string]bool
}

// actor is the caller middleware.LoadCaller put in ctx, or, outside such a
// request, the same user and permissions read in one query.
func (s *service) actor(ctx context.Context, userID int64) (*actor, error) {
	c, ok := user.CallerFrom(ctx, userID)
	if !ok {
		var err error
		if c, err = s.users.GetCaller(ctx, userID); err != nil {
			return nil, fmt.Errorf("get user: %w", err)
		}
		if c == nil {
			return nil, fmt.Errorf("user %d: %w", userID, database.ErrNotFound)
		}
	}
	return &actor{user: c.User, perms: c.Permissions}, nil
}

// canSee reports whether a may act on rep given the _any permission name.
func (a *actor) canSee(rep *report.Report, anyPerm string) bool {
	return rep != nil && (rep.CreatedBy == a.user.ID || a.perms[anyPerm])
}

// redact removes what D-18 keeps from readers who neither own the report nor
// hold write_any or delete_any: the free text, the tithes_offering metric and any
// owner status but deceased (the only one the Archived badge needs).
// The frontend repeats this rule in edit-report.tsx; keep the two in sync.
func (a *actor) redact(resp *report.ReportResponse) {
	if resp.CreatedBy == a.user.ID || a.perms["reports:write_any"] || a.perms["reports:delete_any"] {
		return
	}
	resp.Names, resp.NarrativeReport, resp.Challenges, resp.PrayerRequest = "", "", "", ""
	delete(resp.Metrics, "tithes_offering")
	if resp.OwnerStatus != "deceased" {
		resp.OwnerStatus = ""
	}
}

func (a *actor) auditEntry(ip, action string, rep *report.Report) audit.Log {
	id := strconv.FormatInt(rep.ID, 10)
	name := fmt.Sprintf("%s %d", rep.Month, rep.Year)
	e := audit.Log{
		PerformedBy: &a.user.ID, PerformedByName: &a.user.Name,
		Action: action, TargetType: audit.TargetTypeReport, TargetID: &id, TargetName: &name,
		// Counts and period only: never narrative or prayer text (D-10).
		Details: map[string]any{"month": rep.Month, "year": int(rep.Year), "count": 1},
	}
	if a.user.Role != nil {
		e.PerformedByRole = &a.user.Role.Name
	}
	if ip != "" {
		e.IPAddress = &ip
	}
	return e
}

// load returns the report if the actor may act on it, else report.ErrReportNotFound.
func (s *service) load(ctx context.Context, a *actor, id int64, anyPerm string) (*report.Report, error) {
	rep, err := s.reports.Get(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("get report: %w", err)
	}
	if !a.canSee(rep, anyPerm) {
		return nil, report.ErrReportNotFound
	}
	return rep, nil
}

// mapErr turns repository sentinels into service errors.
func mapErr(err error) error {
	switch {
	case errors.Is(err, database.ErrNotFound):
		return report.ErrReportNotFound
	case errors.Is(err, database.ErrUniqueViolation):
		return report.ErrReportExists
	}
	return err
}

// previousMonth returns the month before now, counted in s.loc.
func (s *service) previousMonth() (string, int) {
	t := s.now().In(s.loc)
	prev := time.Date(t.Year(), t.Month(), 1, 0, 0, 0, 0, s.loc).AddDate(0, -1, 0)
	return report.Months[prev.Month()-1], prev.Year()
}

// Create files a report for userID. A missing year defaults to the current
// year (D-8), or to the previous month's year when that is the month asked for
// (so December filed in January lands in last year). Without reports:write_any
// the period must be the previous month (D-F8).
func (s *service) Create(ctx context.Context, userID int64, ip string, req report.CreateReportRequest) (*report.ReportResponse, error) {
	a, err := s.actor(ctx, userID)
	if err != nil {
		return nil, err
	}
	prevMonth, prevYear := s.previousMonth()
	year := s.now().In(s.loc).Year()
	if req.Month == prevMonth {
		year = prevYear
	}
	if req.Year != nil {
		year = *req.Year
	}
	if !a.perms["reports:write_any"] && (req.Month != prevMonth || year != prevYear) {
		return nil, report.ErrPeriodNotAllowed
	}

	var rep *report.Report
	err = s.tx.InTransaction(ctx, func(ctx context.Context) error {
		var err error
		if rep, err = s.reports.Create(ctx, userID, year, req); err != nil {
			return err
		}
		return s.audit.Insert(ctx, a.auditEntry(ip, audit.ActionCreateReport, rep))
	})
	if err != nil {
		return nil, mapErr(err)
	}
	return respond(rep), nil
}

// Get returns one report. Without reports:read_any only the owner's report is visible.
func (s *service) Get(ctx context.Context, userID, id int64) (*report.ReportResponse, error) {
	a, err := s.actor(ctx, userID)
	if err != nil {
		return nil, err
	}
	rep, err := s.load(ctx, a, id, "reports:read_any")
	if err != nil {
		return nil, err
	}
	resp := respond(rep)
	a.redact(resp)
	return resp, nil
}

// List returns one page. Without reports:read_any the list is limited to the caller's own reports.
func (s *service) List(ctx context.Context, userID int64, p report.ListReportsParams, pg pagination.Params) ([]report.ReportResponse, int64, error) {
	a, err := s.actor(ctx, userID)
	if err != nil {
		return nil, 0, err
	}
	if !a.perms["reports:read_any"] {
		p.CreatedBy = userID
	}
	reports, total, err := s.reports.List(ctx, p, pg)
	if err != nil {
		return nil, 0, fmt.Errorf("list reports: %w", err)
	}
	out := make([]report.ReportResponse, 0, len(reports))
	for _, r := range reports {
		resp := r.ToResponse()
		a.redact(&resp)
		out = append(out, resp)
	}
	return out, total, nil
}

// Update replaces the scalar fields and the whole metric set (D-5). Empty
// strings become NULL in the repository (D-8). A missing year keeps the
// report's current year.
func (s *service) Update(ctx context.Context, userID int64, ip string, id int64, req report.UpdateReportRequest) (*report.ReportResponse, error) {
	a, err := s.actor(ctx, userID)
	if err != nil {
		return nil, err
	}
	existing, err := s.load(ctx, a, id, "reports:write_any")
	if err != nil {
		return nil, err
	}
	year := int(existing.Year)
	if req.Year != nil {
		year = *req.Year
	}

	var rep *report.Report
	err = s.tx.InTransaction(ctx, func(ctx context.Context) error {
		var err error
		if rep, err = s.reports.Update(ctx, id, year, req); err != nil {
			return err
		}
		return s.audit.Insert(ctx, a.auditEntry(ip, audit.ActionUpdateReport, rep))
	})
	if err != nil {
		return nil, mapErr(err)
	}
	return respond(rep), nil
}

// Delete soft-deletes a report. Without reports:delete_any only the owner's can be deleted.
func (s *service) Delete(ctx context.Context, userID int64, ip string, id int64) error {
	a, err := s.actor(ctx, userID)
	if err != nil {
		return err
	}
	rep, err := s.load(ctx, a, id, "reports:delete_any")
	if err != nil {
		return err
	}
	err = s.tx.InTransaction(ctx, func(ctx context.Context) error {
		if err := s.reports.SoftDelete(ctx, id); err != nil {
			return err
		}
		return s.audit.Insert(ctx, a.auditEntry(ip, audit.ActionDeleteReport, rep))
	})
	return mapErr(err)
}

// ToggleComplete approves an open report or unapproves a completed one. Approval
// is global (D-1): any report by ID, gated by reports:approve at the route. It
// returns metadata only, so approve never discloses report text; clients fetch
// the report with Get if they may read it.
func (s *service) ToggleComplete(ctx context.Context, userID int64, ip string, id int64, adminNotes ...string) (*report.ReportMetadata, error) {
	a, err := s.actor(ctx, userID)
	if err != nil {
		return nil, err
	}
	var rep *report.Report
	err = s.tx.InTransaction(ctx, func(ctx context.Context) error {
		var err error
		if rep, err = s.reports.ToggleComplete(ctx, id, userID, adminNotes...); err != nil {
			return err
		}
		action := audit.ActionUnapproveReport
		if rep.Completed {
			action = audit.ActionApproveReport
		}
		return s.audit.Insert(ctx, a.auditEntry(ip, action, rep))
	})
	if err != nil {
		return nil, mapErr(err)
	}
	return &report.ReportMetadata{ID: rep.ID, Completed: rep.Completed, Status: rep.Status, CompletedAt: rep.CompletedAt, CompletedBy: rep.CompletedBy}, nil
}

func (s *service) RequestRevision(ctx context.Context, userID int64, ip string, id int64, adminNotes ...string) (*report.ReportMetadata, error) {
	a, err := s.actor(ctx, userID)
	if err != nil {
		return nil, err
	}
	var rep *report.Report
	err = s.tx.InTransaction(ctx, func(ctx context.Context) error {
		var err error
		if rep, err = s.reports.RequestRevision(ctx, id, userID, adminNotes...); err != nil {
			return err
		}
		return s.audit.Insert(ctx, a.auditEntry(ip, audit.ActionUnapproveReport, rep))
	})
	if err != nil {
		return nil, mapErr(err)
	}
	return &report.ReportMetadata{ID: rep.ID, Completed: rep.Completed, Status: rep.Status, CompletedAt: rep.CompletedAt, CompletedBy: rep.CompletedBy}, nil
}

func respond(r *report.Report) *report.ReportResponse {
	resp := r.ToResponse()
	return &resp
}

// ExportOne builds one report's workbook. The workbook is complete before the
// audit row is written, so a failed build never leaves an audit row (D-10).
func (s *service) ExportOne(ctx context.Context, userID int64, ip string, id int64) (*report.Export, error) {
	a, rep, err := s.oneForExport(ctx, userID, id)
	if err != nil {
		return nil, err
	}
	data, err := workbookBytes([]*report.Report{rep})
	if err != nil {
		return nil, err
	}
	if err := s.audit.Insert(ctx, a.auditEntry(ip, audit.ActionDownloadSingle, rep)); err != nil {
		return nil, fmt.Errorf("insert audit: %w", err)
	}
	label := strings.TrimSpace(deref(rep.ChurchName))
	if label == "" {
		label = "Report " + strconv.FormatInt(rep.ID, 10)
	}
	return &report.Export{Filename: exportFilename(label, rep.Month, int(rep.Year)), Data: data}, nil
}

// ExportMonth builds the month's workbook: the Report Dashboard, then a sheet
// per approved report in monthForExport's order (D-A16). The audit count is
// the number of report sheets, which may be 0.
func (s *service) ExportMonth(ctx context.Context, userID int64, ip string, req report.ExportReportsRequest) (*report.Export, error) {
	a, rows, reps, err := s.monthForExport(ctx, userID, req.Month, req.Year)
	if err != nil {
		return nil, err
	}
	data, err := bulkWorkbookBytes(rows, reps)
	if err != nil {
		return nil, err
	}
	e := a.auditEntry(ip, audit.ActionDownloadBulk, &report.Report{Month: req.Month, Year: int16(req.Year)}) // #nosec G115 - year validated 2000..2100
	e.TargetID, e.Details["count"] = nil, len(reps)
	if err := s.audit.Insert(ctx, e); err != nil {
		return nil, fmt.Errorf("insert audit: %w", err)
	}
	return &report.Export{Filename: exportFilename("Church Growth Program Report", req.Month, req.Year), Data: data}, nil
}

// Preview builds the same workbook as ExportOne (report_id) or ExportMonth
// (month and year) and returns its sheets as text. It never writes an audit row:
// only the download does (D-A15).
func (s *service) Preview(ctx context.Context, userID int64, req report.ExcelPreviewRequest) (*report.ExcelPreview, error) {
	var reps []*report.Report
	var f *excelize.File
	if req.ReportID != 0 {
		_, rep, err := s.oneForExport(ctx, userID, req.ReportID)
		if err != nil {
			return nil, err
		}
		reps = []*report.Report{rep}
		f, err = buildWorkbook(reps)
		if err != nil {
			return nil, fmt.Errorf("build workbook: %w", err)
		}
	} else {
		_, rows, monthReps, err := s.monthForExport(ctx, userID, req.Month, req.Year)
		if err != nil {
			return nil, err
		}
		reps = monthReps
		if f, err = buildBulkWorkbook(rows, reps); err != nil {
			return nil, fmt.Errorf("build workbook: %w", err)
		}
	}
	defer func() { _ = f.Close() }()
	sheets, err := previewSheets(f, req.ReportID == 0)
	if err != nil {
		return nil, fmt.Errorf("preview workbook: %w", err)
	}
	return &report.ExcelPreview{Count: len(reps), Sheets: sheets}, nil
}

// oneForExport loads the report of a single export or preview.
func (s *service) oneForExport(ctx context.Context, userID, id int64) (*actor, *report.Report, error) {
	a, err := s.actor(ctx, userID)
	if err != nil {
		return nil, nil, err
	}
	rep, err := s.load(ctx, a, id, "reports:read_any")
	if err != nil {
		return nil, nil, err
	}
	return a, rep, nil
}

// monthForExport returns the Report Dashboard rows and the approved reports of
// a bulk export or preview, ordered by church name (case-insensitive), then id
// (D-A16). Without reports:read_any the reports are only the caller's own, as
// List does, and the dashboard is empty. More than MaxExportReports approved reports is ErrTooManyReports.
func (s *service) monthForExport(ctx context.Context, userID int64, month string, year int) (*actor, []report.DashboardRow, []*report.Report, error) {
	a, err := s.actor(ctx, userID)
	if err != nil {
		return nil, nil, nil, err
	}
	approved := true
	p := report.ListReportsParams{Month: month, Year: year, Completed: &approved}
	if !a.perms["reports:read_any"] {
		p.CreatedBy = userID
	}
	reps, total, err := s.reports.List(ctx, p, pagination.Params{Page: 1, Limit: report.MaxExportReports})
	if err != nil {
		return nil, nil, nil, fmt.Errorf("list reports: %w", err)
	}
	if total > report.MaxExportReports {
		return nil, nil, nil, report.ErrTooManyReports
	}
	slices.SortFunc(reps, func(x, y *report.Report) int {
		return cmp.Or(cmp.Compare(strings.ToLower(deref(x.ChurchName)), strings.ToLower(deref(y.ChurchName))), cmp.Compare(x.ID, y.ID))
	})
	// The dashboard covers every account, so it follows the same _any scope as
	// the reports: without reports:read_any it is the header alone.
	rows := []report.DashboardRow{}
	if a.perms["reports:read_any"] {
		if rows, err = s.reports.Dashboard(ctx, month, year); err != nil {
			return nil, nil, nil, fmt.Errorf("dashboard: %w", err)
		}
	}
	return a, rows, reps, nil
}

func workbookBytes(reps []*report.Report) ([]byte, error) {
	f, err := buildWorkbook(reps)
	if err != nil {
		return nil, fmt.Errorf("build workbook: %w", err)
	}
	return fileBytes(f)
}

func bulkWorkbookBytes(rows []report.DashboardRow, reps []*report.Report) ([]byte, error) {
	f, err := buildBulkWorkbook(rows, reps)
	if err != nil {
		return nil, fmt.Errorf("build workbook: %w", err)
	}
	return fileBytes(f)
}

func fileBytes(f *excelize.File) ([]byte, error) {
	defer func() { _ = f.Close() }() // only removes temp files; the bytes are already in buf
	buf, err := f.WriteToBuffer()
	if err != nil {
		return nil, fmt.Errorf("write workbook: %w", err)
	}
	return buf.Bytes(), nil
}
