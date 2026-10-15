package service

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"time"

	"github.com/ssr0016/pastoral-reports/backend/internal/model"
	"github.com/ssr0016/pastoral-reports/backend/internal/repository"
	"github.com/ssr0016/pastoral-reports/backend/pkg/pagination"
)

var (
	// ErrReportNotFound is returned for a missing, soft-deleted or someone
	// else's report. The caller maps it to 404 so existence is not revealed (D-1).
	ErrReportNotFound = errors.New("report not found")
	// ErrReportExists is returned when the owner already has an active report
	// for that month and year. The caller maps it to 409.
	ErrReportExists = errors.New("a report for this month and year already exists")
	// ErrPeriodNotAllowed is returned when a caller without reports:write_any
	// files a report for any month but the previous one. The caller maps it to 422.
	ErrPeriodNotAllowed = errors.New("reports can only be created for the previous month")
)

// ReportService holds the report business rules: year default, ownership scope
// (D-1), and one transaction per mutation covering report, metrics and audit
// (D-10, D-14). Route-level RequirePermission decides who may call a method at
// all; this service only applies the _any scope.
//
// Note the repository asymmetry it hides: ReportRepository.Get returns
// (nil, nil) for a missing report, while Update, SoftDelete and ToggleComplete
// return ErrNotFound. Both become ErrReportNotFound here.
type ReportService struct {
	reports repository.ReportRepository
	audit   repository.AuditRepository
	tx      repository.TxRunner
	q       repository.Querier // pool, for reads outside a transaction
	users   repository.UserRepository
	roles   repository.RoleRepository
	now     func() time.Time
	loc     *time.Location // months are counted in Philippine time, whatever the server clock says
}

func NewReportService(
	reports repository.ReportRepository,
	audit repository.AuditRepository,
	tx repository.TxRunner,
	q repository.Querier,
	users repository.UserRepository,
	roles repository.RoleRepository,
) *ReportService {
	loc, err := time.LoadLocation("Asia/Manila")
	if err != nil {
		panic(fmt.Sprintf("load Asia/Manila (is tzdata installed?): %v", err))
	}
	return &ReportService{reports: reports, audit: audit, tx: tx, q: q, users: users, roles: roles, now: time.Now, loc: loc}
}

type actor struct {
	user  *model.User
	perms map[string]bool
}

func (s *ReportService) actor(ctx context.Context, userID int64) (*actor, error) {
	user, err := s.users.GetWithRole(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("get user: %w", err)
	}
	if user == nil {
		return nil, fmt.Errorf("user %d: %w", userID, repository.ErrNotFound)
	}
	a := &actor{user: user, perms: map[string]bool{}}
	if user.RoleID != 0 {
		perms, err := s.roles.GetPermissions(ctx, user.RoleID)
		if err != nil {
			return nil, fmt.Errorf("get permissions: %w", err)
		}
		for _, p := range perms {
			a.perms[p.Name] = true
		}
	}
	return a, nil
}

// canSee reports whether a may act on rep given the _any permission name.
func (a *actor) canSee(rep *model.Report, anyPerm string) bool {
	return rep != nil && (rep.CreatedBy == a.user.ID || a.perms[anyPerm])
}

func (a *actor) auditEntry(ip, action string, rep *model.Report) model.AuditLog {
	id := strconv.FormatInt(rep.ID, 10)
	name := fmt.Sprintf("%s %d", rep.Month, rep.Year)
	e := model.AuditLog{
		PerformedBy: &a.user.ID, PerformedByName: &a.user.Name,
		Action: action, TargetType: model.AuditTargetTypeReport, TargetID: &id, TargetName: &name,
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

// load returns the report if the actor may act on it, else ErrReportNotFound.
func (s *ReportService) load(ctx context.Context, a *actor, id int64, anyPerm string) (*model.Report, error) {
	rep, err := s.reports.Get(ctx, s.q, id)
	if err != nil {
		return nil, fmt.Errorf("get report: %w", err)
	}
	if !a.canSee(rep, anyPerm) {
		return nil, ErrReportNotFound
	}
	return rep, nil
}

// mapErr turns repository sentinels into service errors.
func mapErr(err error) error {
	switch {
	case errors.Is(err, repository.ErrNotFound):
		return ErrReportNotFound
	case errors.Is(err, repository.ErrUniqueViolation):
		return ErrReportExists
	}
	return err
}

// previousMonth returns the month before now, counted in s.loc.
func (s *ReportService) previousMonth() (string, int) {
	t := s.now().In(s.loc)
	prev := time.Date(t.Year(), t.Month(), 1, 0, 0, 0, 0, s.loc).AddDate(0, -1, 0)
	return model.Months[prev.Month()-1], prev.Year()
}

// Create files a report for userID. A missing year defaults to the current
// year (D-8), or to the previous month's year when that is the month asked for
// (so December filed in January lands in last year). Without reports:write_any
// the period must be the previous month (D-F8).
func (s *ReportService) Create(ctx context.Context, userID int64, ip string, req model.CreateReportRequest) (*model.ReportResponse, error) {
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
		return nil, ErrPeriodNotAllowed
	}

	var rep *model.Report
	err = s.tx.WithTx(ctx, func(q repository.Querier) error {
		var err error
		if rep, err = s.reports.Create(ctx, q, userID, year, req); err != nil {
			return err
		}
		return s.audit.Insert(ctx, q, a.auditEntry(ip, model.AuditCreateReport, rep))
	})
	if err != nil {
		return nil, mapErr(err)
	}
	return respond(rep), nil
}

// Get returns one report. Without reports:read_any only the owner's report is visible.
func (s *ReportService) Get(ctx context.Context, userID, id int64) (*model.ReportResponse, error) {
	a, err := s.actor(ctx, userID)
	if err != nil {
		return nil, err
	}
	rep, err := s.load(ctx, a, id, "reports:read_any")
	if err != nil {
		return nil, err
	}
	return respond(rep), nil
}

// List returns one page. Without reports:read_any the list is limited to the caller's own reports.
func (s *ReportService) List(ctx context.Context, userID int64, p model.ListReportsParams, pg pagination.Params) ([]model.ReportResponse, int64, error) {
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
	out := make([]model.ReportResponse, 0, len(reports))
	for _, r := range reports {
		out = append(out, r.ToResponse())
	}
	return out, total, nil
}

// Update replaces the scalar fields and the whole metric set (D-5). Empty
// strings become NULL in the repository (D-8). A missing year keeps the
// report's current year.
func (s *ReportService) Update(ctx context.Context, userID int64, ip string, id int64, req model.UpdateReportRequest) (*model.ReportResponse, error) {
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

	var rep *model.Report
	err = s.tx.WithTx(ctx, func(q repository.Querier) error {
		var err error
		if rep, err = s.reports.Update(ctx, q, id, year, req); err != nil {
			return err
		}
		return s.audit.Insert(ctx, q, a.auditEntry(ip, model.AuditUpdateReport, rep))
	})
	if err != nil {
		return nil, mapErr(err)
	}
	return respond(rep), nil
}

// Delete soft-deletes a report. Without reports:delete_any only the owner's can be deleted.
func (s *ReportService) Delete(ctx context.Context, userID int64, ip string, id int64) error {
	a, err := s.actor(ctx, userID)
	if err != nil {
		return err
	}
	rep, err := s.load(ctx, a, id, "reports:delete_any")
	if err != nil {
		return err
	}
	err = s.tx.WithTx(ctx, func(q repository.Querier) error {
		if err := s.reports.SoftDelete(ctx, q, id); err != nil {
			return err
		}
		return s.audit.Insert(ctx, q, a.auditEntry(ip, model.AuditDeleteReport, rep))
	})
	return mapErr(err)
}

// ToggleComplete approves an open report or unapproves a completed one. Approval
// is global (D-1): any report by ID, gated by reports:approve at the route. It
// returns metadata only, so approve never discloses report text; clients fetch
// the report with Get if they may read it.
func (s *ReportService) ToggleComplete(ctx context.Context, userID int64, ip string, id int64) (*model.ReportMetadata, error) {
	a, err := s.actor(ctx, userID)
	if err != nil {
		return nil, err
	}
	var rep *model.Report
	err = s.tx.WithTx(ctx, func(q repository.Querier) error {
		var err error
		if rep, err = s.reports.ToggleComplete(ctx, q, id, userID); err != nil {
			return err
		}
		action := model.AuditUnapproveReport
		if rep.Completed {
			action = model.AuditApproveReport
		}
		return s.audit.Insert(ctx, q, a.auditEntry(ip, action, rep))
	})
	if err != nil {
		return nil, mapErr(err)
	}
	return &model.ReportMetadata{ID: rep.ID, Completed: rep.Completed, CompletedAt: rep.CompletedAt, CompletedBy: rep.CompletedBy}, nil
}

func respond(r *model.Report) *model.ReportResponse {
	resp := r.ToResponse()
	return &resp
}
