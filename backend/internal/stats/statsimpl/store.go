package statsimpl

import (
	"context"
	"fmt"

	sq "github.com/Masterminds/squirrel"

	"github.com/ssr0016/pastoral-reports/backend/internal/database"
	"github.com/ssr0016/pastoral-reports/backend/internal/report"
	"github.com/ssr0016/pastoral-reports/backend/internal/stats"
)

type store struct{ db *database.DB }

// NewStore returns the stats.Store over db.
func NewStore(db *database.DB) stats.Store { return &store{db: db} }

// Counts returns the admin dashboard numbers for one month in a single read.
// Soft-deleted reports and accounts are not counted (D-A24). The month matches case-insensitively, as
// in the reports repository; month and year are echoed back as given.
func (r *store) Counts(ctx context.Context, month string, year int) (stats.AdminStats, error) {
	inPeriod := "FROM reports WHERE deleted_at IS NULL AND year = ? AND month = ?"
	stored := report.CanonicalMonth(month)
	query, args, err := r.db.Builder.Select().
		Column(sq.Expr("(SELECT COUNT(*) "+inPeriod+")", year, stored)).
		Column(sq.Expr("(SELECT COUNT(*) "+inPeriod+" AND status = 'approved')", year, stored)).
		Column(sq.Expr("(SELECT COUNT(*) "+inPeriod+" AND status = 'needs_revision')", year, stored)).
		Column("(SELECT COUNT(*) FROM users WHERE deleted_at IS NULL)").
		Column(sq.Expr("(SELECT COUNT(*) FROM users u JOIN roles ro ON ro.id = u.role_id WHERE u.deleted_at IS NULL AND ro.name = ?)", "admin")).
		Column(sq.Expr("(SELECT COUNT(*) FROM users u JOIN roles ro ON ro.id = u.role_id WHERE ro.name = 'user' AND u.deleted_at IS NULL AND u.status = 'active' AND btrim(COALESCE(u.church_name, '')) <> '')")).
		Column(sq.Expr("(SELECT COUNT(*) FROM users u JOIN roles ro ON ro.id = u.role_id WHERE ro.name = 'user' AND u.deleted_at IS NULL AND u.status = 'active' AND btrim(COALESCE(u.church_name, '')) <> '' AND NOT EXISTS (SELECT 1 FROM reports r WHERE r.created_by = u.id AND r.deleted_at IS NULL AND r.year = ? AND r.month = ?))", year, stored)).
		Column(sq.Expr("(SELECT COUNT(DISTINCT lower(btrim(u.church_name))) FROM users u JOIN roles ro ON ro.id = u.role_id WHERE ro.name = 'user' AND u.deleted_at IS NULL AND u.status = 'active' AND btrim(COALESCE(u.church_name, '')) <> '')")).
		ToSql()
	if err != nil {
		return stats.AdminStats{}, fmt.Errorf("build stats query: %w", err)
	}
	s := stats.AdminStats{Month: month, Year: year}
	if err := r.db.Querier(ctx).QueryRow(ctx, query, args...).Scan(&s.ReportsTotal, &s.ReportsApproved, &s.ReportsNeedsRevision, &s.UsersTotal, &s.AdminsTotal, &s.ExpectedReports, &s.NoReport, &s.ChurchesTotal); err != nil {
		return stats.AdminStats{}, fmt.Errorf("count stats: %w", err)
	}
	s.ReportsPending = s.ReportsTotal - s.ReportsApproved - s.ReportsNeedsRevision
	return s, nil
}

// Tracker reads each church account (report.ChurchAccount) and its one active report for the month
// (a unique index allows one per account and period) in a single query (D-A3).
func (r *store) Tracker(ctx context.Context, month string, year int) ([]stats.TrackerRow, error) {
	query, args, err := r.db.Builder.
		Select("u.id", "u.name", "u.email", "COALESCE(u.church_name, '')", "u.status", "r.status").
		From("users u").
		Join("roles ro ON ro.id = u.role_id").
		LeftJoin("reports r ON r.created_by = u.id AND r.deleted_at IS NULL AND r.year = ? AND r.month = ?", year, report.CanonicalMonth(month)).
		Where(report.ChurchAccount).
		OrderBy("lower(u.church_name)", "lower(u.name)", "u.id").
		ToSql()
	if err != nil {
		return nil, fmt.Errorf("build tracker query: %w", err)
	}
	rows, err := r.db.Querier(ctx).Query(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("tracker: %w", err)
	}
	defer rows.Close()
	out := []stats.TrackerRow{}
	for rows.Next() {
		var row stats.TrackerRow
		var status *string // NULL when there is no report this month
		if err := rows.Scan(&row.UserID, &row.Name, &row.Email, &row.ChurchName, &row.UserStatus, &status); err != nil {
			return nil, fmt.Errorf("scan tracker: %w", err)
		}
		if status == nil {
			row.SubmissionStatus = stats.TrackerNoReport
		} else {
			row.SubmissionStatus = stats.TrackerSubmitted
			switch *status {
			case "approved":
				row.ReviewStatus = stats.TrackerApproved
			case "needs_revision":
				row.ReviewStatus = stats.TrackerNeedsRevision
			default:
				row.ReviewStatus = stats.TrackerPending
			}
		}
		out = append(out, row)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("tracker: %w", err)
	}
	return out, nil
}

func (r *store) AnalyticsAttendance(ctx context.Context, q stats.AttendanceQuery) (stats.AttendanceAnalytics, error) {
	statusWhere := "AND r.status = 'approved'"
	if q.Status == stats.AttendanceStatusAll {
		statusWhere = "AND r.status IN ('pending', 'needs_revision', 'approved')"
	}
	trendQuery, trendArgs, err := r.db.Builder.
		Select("EXTRACT(MONTH FROM to_date(r.month, 'Month'))::int", "rm.week_no", "SUM(rm.value)::double precision").
		From("reports r").
		Join("report_metrics rm ON rm.report_id = r.id").
		Where("r.deleted_at IS NULL AND r.year = ? AND rm.metric = ? "+statusWhere, q.Year, q.Metric).
		GroupBy("1", "2").
		OrderBy("1", "2").
		ToSql()
	if err != nil {
		return stats.AttendanceAnalytics{}, fmt.Errorf("build attendance trend query: %w", err)
	}
	rows, err := r.db.Querier(ctx).Query(ctx, trendQuery, trendArgs...)
	if err != nil {
		return stats.AttendanceAnalytics{}, fmt.Errorf("query attendance trend: %w", err)
	}
	defer rows.Close()
	values := make(map[[2]int]float64)
	for rows.Next() {
		var month, week int
		var value float64
		if err := rows.Scan(&month, &week, &value); err != nil {
			return stats.AttendanceAnalytics{}, fmt.Errorf("scan attendance trend: %w", err)
		}
		values[[2]int{month, week}] = value
	}
	if err := rows.Err(); err != nil {
		return stats.AttendanceAnalytics{}, fmt.Errorf("attendance trend: %w", err)
	}

	churchQuery, churchArgs, err := r.db.Builder.
		Select("COALESCE(NULLIF(btrim(r.church_name), ''), 'Unassigned')", "SUM(rm.value)::double precision").
		From("reports r").
		Join("report_metrics rm ON rm.report_id = r.id").
		Where("r.deleted_at IS NULL AND r.year = ? AND rm.metric = ? "+statusWhere, q.Year, q.Metric).
		GroupBy("1").
		OrderBy("1").
		ToSql()
	if err != nil {
		return stats.AttendanceAnalytics{}, fmt.Errorf("build attendance church query: %w", err)
	}
	churchRows, err := r.db.Querier(ctx).Query(ctx, churchQuery, churchArgs...)
	if err != nil {
		return stats.AttendanceAnalytics{}, fmt.Errorf("query attendance churches: %w", err)
	}
	defer churchRows.Close()
	churches := []stats.AttendanceChurch{}
	for churchRows.Next() {
		var church stats.AttendanceChurch
		if err := churchRows.Scan(&church.ChurchName, &church.Value); err != nil {
			return stats.AttendanceAnalytics{}, fmt.Errorf("scan attendance church: %w", err)
		}
		churches = append(churches, church)
	}
	if err := churchRows.Err(); err != nil {
		return stats.AttendanceAnalytics{}, fmt.Errorf("attendance churches: %w", err)
	}

	trend := make([]stats.AttendanceTrendPoint, 0, 60)
	for month, name := range report.Months {
		for week := 1; week <= 5; week++ {
			trend = append(trend, stats.AttendanceTrendPoint{Month: name, Week: week, Value: values[[2]int{month + 1, week}]})
		}
	}
	return stats.AttendanceAnalytics{Trend: trend, Churches: churches}, nil
}
