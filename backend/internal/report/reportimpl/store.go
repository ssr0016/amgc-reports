package reportimpl

import (
	"context"
	"errors"
	"fmt"
	"strings"

	sq "github.com/Masterminds/squirrel"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"

	"github.com/ssr0016/pastoral-reports/backend/internal/database"
	"github.com/ssr0016/pastoral-reports/backend/internal/report"
	"github.com/ssr0016/pastoral-reports/backend/pkg/pagination"
)

// store reads and writes reports and their metrics through the context's transaction.
// Queries exclude soft-deleted reports; deleted owners never hide their reports.
// A write that duplicates (created_by, month, year) wraps database.ErrUniqueViolation.
type store struct{ db *database.DB }

// NewStore returns the report.Store over db.
func NewStore(db *database.DB) report.Store { return &store{db: db} }

const reportColumns = "id, created_by, month, year, worker, area_assignment, church_name, names, " +
	"narrative_report, challenges, prayer_request, completed, status, admin_notes, last_revised_at, user_notes, completed_at, completed_by, " +
	"deleted_at, created_at, updated_at"

// reportColumnsWithAlias prefixes every column with alias, e.g. "r.id".
func reportColumnsWithAlias(alias string) string {
	return alias + "." + strings.ReplaceAll(reportColumns, ", ", ", "+alias+".")
}

// escapeLike makes %, _ and \ match literally inside an ILIKE pattern.
func escapeLike(s string) string {
	return strings.NewReplacer(`\`, `\\`, `%`, `\%`, `_`, `\_`).Replace(s)
}

// nullable stores an empty string as NULL (D-8).
func nullable(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}

func scanReport(row pgx.Row, withOwner bool) (*report.Report, error) {
	var r report.Report
	dest := []any{&r.ID, &r.CreatedBy, &r.Month, &r.Year, &r.Worker, &r.AreaAssignment,
		&r.ChurchName, &r.Names, &r.NarrativeReport, &r.Challenges, &r.PrayerRequest,
		&r.Completed, &r.Status, &r.AdminNotes, &r.LastRevisedAt, &r.UserNotes,
		&r.CompletedAt, &r.CompletedBy, &r.DeletedAt, &r.CreatedAt, &r.UpdatedAt}
	if withOwner {
		dest = append(dest, &r.OwnerStatus, &r.OwnerDeletedAt)
	}
	err := row.Scan(dest...)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if database.IsUniqueViolation(err) {
		return nil, fmt.Errorf("scan report: %w: %w", database.ErrUniqueViolation, err)
	}
	if err != nil {
		return nil, fmt.Errorf("scan report: %w", err)
	}
	return &r, nil
}

// Create inserts the report and its metrics and returns it with Metrics loaded.
// year is already resolved by the caller (the request may omit it).
func (r *store) Create(ctx context.Context, userID int64, year int, req report.CreateReportRequest) (*report.Report, error) {
	query, args, err := r.db.Builder.
		Insert("reports").
		Columns("created_by", "month", "year", "worker", "area_assignment", "church_name",
			"names", "narrative_report", "challenges", "prayer_request").
		Values(userID, req.Month, year, nullable(req.Worker), nullable(req.AreaAssignment), nullable(req.ChurchName),
			nullable(req.Names), nullable(req.NarrativeReport), nullable(req.Challenges), nullable(req.PrayerRequest)).
		Suffix("RETURNING " + reportColumns).
		ToSql()
	if err != nil {
		return nil, fmt.Errorf("build query: %w", err)
	}
	rep, err := scanReport(r.db.Querier(ctx).QueryRow(ctx, query, args...), false)
	if err != nil {
		return nil, err
	}
	if err := r.insertMetrics(ctx, rep.ID, req.Metrics); err != nil {
		return nil, err
	}
	return r.Get(ctx, rep.ID)
}

// insertMetrics writes every supplied (metric, week) in one multi-row insert.
// Values go in as text and are cast to NUMERIC by Postgres (D-4).
func (r *store) insertMetrics(ctx context.Context, reportID int64, metrics map[string]report.MetricWeeks) error {
	ib := r.db.Builder.Insert("report_metrics").Columns("report_id", "metric", "week_no", "value")
	n := 0
	for metric, weeks := range metrics {
		for weekNo, v := range weeks.NonNil() {
			ib = ib.Values(reportID, metric, weekNo, sq.Expr("?::numeric", string(v)))
			n++
		}
	}
	if n == 0 {
		return nil
	}
	query, args, err := ib.ToSql()
	if err != nil {
		return fmt.Errorf("build query: %w", err)
	}
	if _, err := r.db.Querier(ctx).Exec(ctx, query, args...); err != nil {
		return fmt.Errorf("insert metrics: %w", err)
	}
	return nil
}

// loadMetrics fills Metrics on all given reports with one query (no N+1).
func (r *store) loadMetrics(ctx context.Context, reports []*report.Report) error {
	if len(reports) == 0 {
		return nil
	}
	byID := make(map[int64]*report.Report, len(reports))
	ids := make([]int64, 0, len(reports))
	for _, rep := range reports {
		rep.Metrics = map[string]map[int]pgtype.Numeric{}
		byID[rep.ID] = rep
		ids = append(ids, rep.ID)
	}
	query, args, err := r.db.Builder.
		Select("report_id", "metric", "week_no", "value").
		From("report_metrics").
		Where(sq.Eq{"report_id": ids}).
		ToSql()
	if err != nil {
		return fmt.Errorf("build query: %w", err)
	}
	rows, err := r.db.Querier(ctx).Query(ctx, query, args...)
	if err != nil {
		return fmt.Errorf("query metrics: %w", err)
	}
	defer rows.Close()
	for rows.Next() {
		var m report.ReportMetric
		if err := rows.Scan(&m.ReportID, &m.Metric, &m.WeekNo, &m.Value); err != nil {
			return fmt.Errorf("scan metric: %w", err)
		}
		rep := byID[m.ReportID]
		if rep.Metrics[m.Metric] == nil {
			rep.Metrics[m.Metric] = map[int]pgtype.Numeric{}
		}
		rep.Metrics[m.Metric][int(m.WeekNo)] = m.Value
	}
	return rows.Err()
}

// Get returns the report with metrics, or nil, nil when it is missing or soft-deleted.
func (r *store) Get(ctx context.Context, id int64) (*report.Report, error) {
	query, args, err := r.db.Builder.
		Select(reportColumnsWithAlias("r"), "u.status", "u.deleted_at").
		From("reports r").
		Join("users u ON u.id = r.created_by").
		Where(sq.Eq{"r.id": id, "r.deleted_at": nil}).
		ToSql()
	if err != nil {
		return nil, fmt.Errorf("build query: %w", err)
	}
	rep, err := scanReport(r.db.Querier(ctx).QueryRow(ctx, query, args...), true)
	if err != nil || rep == nil {
		return nil, err
	}
	if err := r.loadMetrics(ctx, []*report.Report{rep}); err != nil {
		return nil, err
	}
	return rep, nil
}

// listWhere is List's filter: not deleted, then each set parameter.
func listWhere(p report.ListReportsParams) sq.And {
	where := sq.And{sq.Eq{"r.deleted_at": nil}}
	if p.Year != 0 {
		where = append(where, sq.Eq{"r.year": p.Year})
	}
	if p.CreatedBy != 0 {
		where = append(where, sq.Eq{"r.created_by": p.CreatedBy})
	}
	if p.Month != "" {
		where = append(where, sq.Eq{"r.month": report.CanonicalMonth(p.Month)})
	}
	if p.Completed != nil {
		where = append(where, sq.Eq{"r.completed": *p.Completed})
	}
	if p.Status != "" {
		where = append(where, sq.Eq{"r.status": p.Status})
	}
	if p.Worker != "" {
		where = append(where, sq.ILike{"r.worker": "%" + escapeLike(p.Worker) + "%"})
	}
	if p.Area != "" {
		where = append(where, sq.ILike{"r.area_assignment": "%" + escapeLike(p.Area) + "%"})
	}
	if p.Church != "" {
		where = append(where, sq.ILike{"r.church_name": "%" + escapeLike(p.Church) + "%"})
	}
	return where
}

// List returns one page of reports (newest first) with metrics, and the total
// count of matching reports.
func (r *store) List(ctx context.Context, p report.ListReportsParams, pg pagination.Params) ([]*report.Report, int64, error) {
	where := listWhere(p)

	countQ, countArgs, err := r.db.Builder.Select("COUNT(*)").From("reports r").Where(where).ToSql()
	if err != nil {
		return nil, 0, fmt.Errorf("build count query: %w", err)
	}
	var total int64
	if err := r.db.Querier(ctx).QueryRow(ctx, countQ, countArgs...).Scan(&total); err != nil {
		return nil, 0, fmt.Errorf("count reports: %w", err)
	}

	listQ, listArgs, err := r.db.Builder.
		Select(reportColumnsWithAlias("r"), "u.status", "u.deleted_at").
		From("reports r").
		Join("users u ON u.id = r.created_by").
		Where(where).
		OrderBy("r.year DESC", "r.created_at DESC", "r.id DESC").
		Limit(uint64(pg.Limit)).     // #nosec G115 - limit validated
		Offset(uint64(pg.Offset())). // #nosec G115 - offset validated
		ToSql()
	if err != nil {
		return nil, 0, fmt.Errorf("build query: %w", err)
	}
	rows, err := r.db.Querier(ctx).Query(ctx, listQ, listArgs...)
	if err != nil {
		return nil, 0, fmt.Errorf("list reports: %w", err)
	}
	defer rows.Close()
	reports := []*report.Report{}
	for rows.Next() {
		rep, err := scanReport(rows, true)
		if err != nil {
			return nil, 0, err
		}
		reports = append(reports, rep)
	}
	if err := rows.Err(); err != nil {
		return nil, 0, fmt.Errorf("list reports: %w", err)
	}
	rows.Close()
	if err := r.loadMetrics(ctx, reports); err != nil {
		return nil, 0, err
	}
	return reports, total, nil
}

// Update replaces the scalar fields and the whole metric set. created_by and
// completed* are never touched. Returns database.ErrNotFound when the report is missing.
func (r *store) Update(ctx context.Context, id int64, year int, req report.UpdateReportRequest) (*report.Report, error) {
	query, args, err := r.db.Builder.
		Update("reports").
		SetMap(map[string]any{
			"month": req.Month, "year": year, "worker": nullable(req.Worker),
			"area_assignment": nullable(req.AreaAssignment), "church_name": nullable(req.ChurchName),
			"names": nullable(req.Names), "narrative_report": nullable(req.NarrativeReport),
			"challenges": nullable(req.Challenges), "prayer_request": nullable(req.PrayerRequest),
			"status":          sq.Expr("CASE WHEN status = 'needs_revision' THEN 'pending' ELSE status END"),
			"last_revised_at": sq.Expr("CASE WHEN status = 'needs_revision' THEN NOW() ELSE last_revised_at END"),
			"user_notes":      sq.Expr("CASE WHEN status = 'needs_revision' THEN ? ELSE user_notes END", nullable(req.UserNotes)),
		}).
		Set("updated_at", sq.Expr("NOW()")).
		Where(sq.Eq{"id": id, "deleted_at": nil}).
		Suffix("RETURNING " + reportColumns).
		ToSql()
	if err != nil {
		return nil, fmt.Errorf("build query: %w", err)
	}
	rep, err := scanReport(r.db.Querier(ctx).QueryRow(ctx, query, args...), false)
	if err != nil {
		return nil, err
	}
	if rep == nil {
		return nil, fmt.Errorf("report: %w", database.ErrNotFound)
	}

	delQ, delArgs, err := r.db.Builder.Delete("report_metrics").Where(sq.Eq{"report_id": id}).ToSql()
	if err != nil {
		return nil, fmt.Errorf("build query: %w", err)
	}
	if _, err := r.db.Querier(ctx).Exec(ctx, delQ, delArgs...); err != nil {
		return nil, fmt.Errorf("delete metrics: %w", err)
	}
	if err := r.insertMetrics(ctx, id, req.Metrics); err != nil {
		return nil, err
	}
	return r.Get(ctx, rep.ID)
}

// SoftDelete marks the report deleted. Returns database.ErrNotFound when it is missing
// or already deleted.
func (r *store) SoftDelete(ctx context.Context, id int64) error {
	query, args, err := r.db.Builder.
		Update("reports").
		Set("deleted_at", sq.Expr("NOW()")).
		Set("updated_at", sq.Expr("NOW()")).
		Where(sq.Eq{"id": id, "deleted_at": nil}).
		ToSql()
	if err != nil {
		return fmt.Errorf("build query: %w", err)
	}
	res, err := r.db.Querier(ctx).Exec(ctx, query, args...)
	if err != nil {
		return fmt.Errorf("soft delete report: %w", err)
	}
	if res.RowsAffected() == 0 {
		return fmt.Errorf("report: %w", database.ErrNotFound)
	}
	return nil
}

// Dashboard joins each church account to its active report of the month.
// idx_reports_unique_per_user allows one such report per account, so each
// account gives one row (D-A16, D-A24).
func (r *store) Dashboard(ctx context.Context, month string, year int) ([]report.DashboardRow, error) {
	q, args, err := r.db.Builder.
		Select("u.church_name", "r.month", "r.year", "r.status").
		From("users u").
		Join("roles ro ON ro.id = u.role_id").
		LeftJoin("reports r ON r.created_by = u.id AND r.deleted_at IS NULL AND r.year = ? AND r.month = ?", year, report.CanonicalMonth(month)).
		Where(report.ChurchAccount).
		OrderBy("lower(u.church_name)", "u.id").
		ToSql()
	if err != nil {
		return nil, fmt.Errorf("build dashboard query: %w", err)
	}
	rows, err := r.db.Querier(ctx).Query(ctx, q, args...)
	if err != nil {
		return nil, fmt.Errorf("dashboard: %w", err)
	}
	defer rows.Close()
	out := []report.DashboardRow{}
	for rows.Next() {
		var row report.DashboardRow
		var month *string // the report columns are NULL when there is no report this month
		var year *int
		var status *string
		if err := rows.Scan(&row.Church, &month, &year, &status); err != nil {
			return nil, fmt.Errorf("scan dashboard: %w", err)
		}
		if month != nil && year != nil {
			row.Month, row.Year = *month, *year
		}
		switch {
		case status == nil:
			row.Status = report.StatusNoReport
		case *status == "approved":
			row.Status = report.StatusApproved
		case *status == "needs_revision":
			row.Status = "Needs Revision"
		default:
			row.Status = report.StatusPending
		}
		out = append(out, row)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("dashboard: %w", err)
	}
	return out, nil
}

// ToggleComplete flips completed in one statement and returns the report; the
// returned Completed says whether this was an approve or an unapprove. Metrics
// are not loaded. Returns database.ErrNotFound when the report is missing.
func (r *store) ToggleComplete(ctx context.Context, id, approverID int64, adminNotes ...string) (*report.Report, error) {
	notes := nullable(firstNote(adminNotes))
	query, args, err := r.db.Builder.
		Update("reports").
		Set("completed", sq.Expr("NOT completed")).
		Set("status", sq.Expr("CASE WHEN completed THEN 'needs_revision' ELSE 'approved' END")).
		Set("completed_at", sq.Expr("CASE WHEN completed THEN NULL ELSE NOW() END")).
		Set("completed_by", sq.Expr("CASE WHEN completed THEN NULL ELSE ?::bigint END", approverID)).
		Set("admin_notes", sq.Expr("CASE WHEN completed THEN ? ELSE NULL END", notes)).
		Set("updated_at", sq.Expr("NOW()")).
		Where(sq.Eq{"id": id, "deleted_at": nil}).
		Suffix("RETURNING " + reportColumns).
		ToSql()
	if err != nil {
		return nil, fmt.Errorf("build query: %w", err)
	}
	rep, err := scanReport(r.db.Querier(ctx).QueryRow(ctx, query, args...), false)
	if err != nil {
		return nil, err
	}
	if rep == nil {
		return nil, fmt.Errorf("report: %w", database.ErrNotFound)
	}
	return rep, nil
}

func (r *store) RequestRevision(ctx context.Context, id, approverID int64, adminNotes ...string) (*report.Report, error) {
	notes := nullable(firstNote(adminNotes))
	query, args, err := r.db.Builder.Update("reports").
		Set("completed", false).
		Set("status", "needs_revision").
		Set("completed_at", nil).
		Set("completed_by", nil).
		Set("admin_notes", notes).
		Set("updated_at", sq.Expr("NOW()")).
		Where(sq.Eq{"id": id, "deleted_at": nil}).
		Suffix("RETURNING " + reportColumns).ToSql()
	if err != nil {
		return nil, fmt.Errorf("build query: %w", err)
	}
	rep, err := scanReport(r.db.Querier(ctx).QueryRow(ctx, query, args...), false)
	if err != nil {
		return nil, err
	}
	if rep == nil {
		return nil, fmt.Errorf("report: %w", database.ErrNotFound)
	}
	return rep, nil
}

func firstNote(notes []string) string {
	if len(notes) == 0 {
		return ""
	}
	return strings.TrimSpace(notes[0])
}
