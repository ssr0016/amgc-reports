package repository

import (
	"context"
	"errors"
	"fmt"
	"strings"

	sq "github.com/Masterminds/squirrel"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"

	"github.com/ssr0016/pastoral-reports/backend/internal/database"
	"github.com/ssr0016/pastoral-reports/backend/internal/model"
	"github.com/ssr0016/pastoral-reports/backend/pkg/pagination"
)

// ReportRepo reads and writes reports and their metrics. Writes take a Querier
// so the service can run them, and the audit insert, in one WithTx. Every query
// excludes soft-deleted rows. A write that would duplicate (created_by, month,
// year) returns an error wrapping ErrUniqueViolation.
type ReportRepo struct{ db *database.DB }

func NewReportRepo(db *database.DB) *ReportRepo { return &ReportRepo{db: db} }

const reportColumns = "id, created_by, month, year, worker, area_assignment, church_name, names, " +
	"narrative_report, challenges, prayer_request, completed, completed_at, completed_by, " +
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

func scanReport(row pgx.Row) (*model.Report, error) {
	var r model.Report
	err := row.Scan(&r.ID, &r.CreatedBy, &r.Month, &r.Year, &r.Worker, &r.AreaAssignment,
		&r.ChurchName, &r.Names, &r.NarrativeReport, &r.Challenges, &r.PrayerRequest,
		&r.Completed, &r.CompletedAt, &r.CompletedBy, &r.DeletedAt, &r.CreatedAt, &r.UpdatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if isUniqueViolation(err) {
		return nil, fmt.Errorf("scan report: %w: %w", ErrUniqueViolation, err)
	}
	if err != nil {
		return nil, fmt.Errorf("scan report: %w", err)
	}
	return &r, nil
}

// Create inserts the report and its metrics and returns it with Metrics loaded.
// year is already resolved by the caller (the request may omit it).
func (r *ReportRepo) Create(ctx context.Context, userID int64, year int, req model.CreateReportRequest) (*model.Report, error) {
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
	rep, err := scanReport(r.db.Querier(ctx).QueryRow(ctx, query, args...))
	if err != nil {
		return nil, err
	}
	if err := r.insertMetrics(ctx, rep.ID, req.Metrics); err != nil {
		return nil, err
	}
	if err := r.loadMetrics(ctx, []*model.Report{rep}); err != nil {
		return nil, err
	}
	return rep, nil
}

// insertMetrics writes every supplied (metric, week) in one multi-row insert.
// Values go in as text and are cast to NUMERIC by Postgres (D-4).
func (r *ReportRepo) insertMetrics(ctx context.Context, reportID int64, metrics map[string]model.MetricWeeks) error {
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
func (r *ReportRepo) loadMetrics(ctx context.Context, reports []*model.Report) error {
	if len(reports) == 0 {
		return nil
	}
	byID := make(map[int64]*model.Report, len(reports))
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
		var m model.ReportMetric
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
func (r *ReportRepo) Get(ctx context.Context, id int64) (*model.Report, error) {
	query, args, err := r.db.Builder.
		Select(reportColumns).
		From("reports").
		Where(sq.Eq{"id": id, "deleted_at": nil}).
		ToSql()
	if err != nil {
		return nil, fmt.Errorf("build query: %w", err)
	}
	rep, err := scanReport(r.db.Querier(ctx).QueryRow(ctx, query, args...))
	if err != nil || rep == nil {
		return nil, err
	}
	if err := r.loadMetrics(ctx, []*model.Report{rep}); err != nil {
		return nil, err
	}
	return rep, nil
}

// List returns one page of reports (newest first) with metrics, and the total
// count of matching reports.
func (r *ReportRepo) List(ctx context.Context, p model.ListReportsParams, pg pagination.Params) ([]*model.Report, int64, error) {
	where := sq.And{sq.Eq{"r.deleted_at": nil}}
	if p.Year != 0 {
		where = append(where, sq.Eq{"r.year": p.Year})
	}
	if p.CreatedBy != 0 {
		where = append(where, sq.Eq{"r.created_by": p.CreatedBy})
	}
	if p.Month != "" {
		where = append(where, sq.Expr("lower(r.month) = lower(?)", p.Month))
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

	countQ, countArgs, err := r.db.Builder.Select("COUNT(*)").From("reports r").Where(where).ToSql()
	if err != nil {
		return nil, 0, fmt.Errorf("build count query: %w", err)
	}
	var total int64
	if err := r.db.Querier(ctx).QueryRow(ctx, countQ, countArgs...).Scan(&total); err != nil {
		return nil, 0, fmt.Errorf("count reports: %w", err)
	}

	listQ, listArgs, err := r.db.Builder.
		Select(reportColumnsWithAlias("r")).
		From("reports r").
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
	reports := []*model.Report{}
	for rows.Next() {
		rep, err := scanReport(rows)
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
// completed* are never touched. Returns ErrNotFound when the report is missing.
func (r *ReportRepo) Update(ctx context.Context, id int64, year int, req model.UpdateReportRequest) (*model.Report, error) {
	query, args, err := r.db.Builder.
		Update("reports").
		SetMap(map[string]any{
			"month": req.Month, "year": year, "worker": nullable(req.Worker),
			"area_assignment": nullable(req.AreaAssignment), "church_name": nullable(req.ChurchName),
			"names": nullable(req.Names), "narrative_report": nullable(req.NarrativeReport),
			"challenges": nullable(req.Challenges), "prayer_request": nullable(req.PrayerRequest),
		}).
		Set("updated_at", sq.Expr("NOW()")).
		Where(sq.Eq{"id": id, "deleted_at": nil}).
		Suffix("RETURNING " + reportColumns).
		ToSql()
	if err != nil {
		return nil, fmt.Errorf("build query: %w", err)
	}
	rep, err := scanReport(r.db.Querier(ctx).QueryRow(ctx, query, args...))
	if err != nil {
		return nil, err
	}
	if rep == nil {
		return nil, fmt.Errorf("report: %w", ErrNotFound)
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
	if err := r.loadMetrics(ctx, []*model.Report{rep}); err != nil {
		return nil, err
	}
	return rep, nil
}

// SoftDelete marks the report deleted. Returns ErrNotFound when it is missing
// or already deleted.
func (r *ReportRepo) SoftDelete(ctx context.Context, id int64) error {
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
		return fmt.Errorf("report: %w", ErrNotFound)
	}
	return nil
}

// ToggleComplete flips completed in one statement and returns the report; the
// returned Completed says whether this was an approve or an unapprove. Metrics
// are not loaded. Returns ErrNotFound when the report is missing.
func (r *ReportRepo) ToggleComplete(ctx context.Context, id, approverID int64) (*model.Report, error) {
	query, args, err := r.db.Builder.
		Update("reports").
		Set("completed", sq.Expr("NOT completed")).
		Set("completed_at", sq.Expr("CASE WHEN completed THEN NULL ELSE NOW() END")).
		Set("completed_by", sq.Expr("CASE WHEN completed THEN NULL ELSE ?::bigint END", approverID)).
		Set("updated_at", sq.Expr("NOW()")).
		Where(sq.Eq{"id": id, "deleted_at": nil}).
		Suffix("RETURNING " + reportColumns).
		ToSql()
	if err != nil {
		return nil, fmt.Errorf("build query: %w", err)
	}
	rep, err := scanReport(r.db.Querier(ctx).QueryRow(ctx, query, args...))
	if err != nil {
		return nil, err
	}
	if rep == nil {
		return nil, fmt.Errorf("report: %w", ErrNotFound)
	}
	return rep, nil
}
