package statsimpl

import (
	"context"
	"errors"
	"fmt"
	"time"

	sq "github.com/Masterminds/squirrel"
	"github.com/jackc/pgx/v5"

	"github.com/ssr0016/pastoral-reports/backend/internal/accesscontrol"
	"github.com/ssr0016/pastoral-reports/backend/internal/database"
	"github.com/ssr0016/pastoral-reports/backend/internal/report"
	"github.com/ssr0016/pastoral-reports/backend/internal/stats"
	"github.com/ssr0016/pastoral-reports/backend/internal/user"
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
		Column("(SELECT COUNT(*) FROM users WHERE deleted_at IS NULL AND status = 'active')").
		Column(sq.Expr("(SELECT COUNT(*) FROM users u JOIN roles ro ON ro.id = u.role_id WHERE u.deleted_at IS NULL AND ro.name = ?)", "admin")).
		Column(sq.Expr("(SELECT COUNT(*) FROM users u JOIN roles ro ON ro.id = u.role_id WHERE ro.name = 'user' AND u.deleted_at IS NULL AND u.status = 'active' AND btrim(COALESCE(u.church_name, '')) <> '')")).
		Column(sq.Expr("(SELECT COUNT(*) FROM users u JOIN roles ro ON ro.id = u.role_id WHERE ro.name = 'user' AND u.deleted_at IS NULL AND u.status = 'active' AND btrim(COALESCE(u.church_name, '')) <> '' AND NOT EXISTS (SELECT 1 FROM reports r WHERE r.created_by = u.id AND r.deleted_at IS NULL AND r.year = ? AND r.month = ?))", year, stored)).
		Column(sq.Expr("(SELECT COUNT(DISTINCT lower(btrim(u.church_name))) FROM users u JOIN roles ro ON ro.id = u.role_id WHERE ro.name = 'user' AND u.deleted_at IS NULL AND u.status = 'active' AND btrim(COALESCE(u.church_name, '')) <> '')")).
		// Roles and statuses come back as JSON arrays so the counts stay one read; zeros are kept.
		Column("(SELECT COALESCE(json_agg(json_build_object('role_id', ro.id, 'name', ro.name, 'count', (SELECT COUNT(*) FROM users u WHERE u.role_id = ro.id AND u.deleted_at IS NULL)) ORDER BY ro.id), '[]') FROM roles ro)").
		Column(sq.Expr("(SELECT json_agg(json_build_object('status', t.s, 'count', (SELECT COUNT(*) FROM users u WHERE u.status = t.s AND u.deleted_at IS NULL)) ORDER BY t.n) FROM unnest(?::text[]) WITH ORDINALITY AS t(s, n))", user.UserStatuses)).
		ToSql()
	if err != nil {
		return stats.AdminStats{}, fmt.Errorf("build stats query: %w", err)
	}
	s := stats.AdminStats{Month: month, Year: year}
	if err := r.db.Querier(ctx).QueryRow(ctx, query, args...).Scan(&s.ReportsTotal, &s.ReportsApproved, &s.ReportsNeedsRevision, &s.UsersTotal, &s.UsersActive, &s.AdminsTotal, &s.ExpectedReports, &s.NoReport, &s.ChurchesTotal, &s.RoleCounts, &s.StatusCounts); err != nil {
		return stats.AdminStats{}, fmt.Errorf("count stats: %w", err)
	}
	s.ReportsPending = s.ReportsTotal - s.ReportsApproved - s.ReportsNeedsRevision
	return s, nil
}

// Tracker reads each church account (report.ChurchAccount) and its one active report for the month
// (a unique index allows one per account and period) in a single query (D-A3).
func (r *store) Tracker(ctx context.Context, month string, year int) ([]stats.TrackerRow, error) {
	query, args, err := r.db.Builder.
		Select("u.id", "u.name", "u.email", "COALESCE(u.church_name, '')", "u.status", "r.status", "u.created_at").
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
		if err := rows.Scan(&row.UserID, &row.Name, &row.Email, &row.ChurchName, &row.UserStatus, &status, &row.CreatedAt); err != nil {
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

const churchesQuery = `
SELECT c.id, c.name, c.reporting_start_month, c.reporting_end_month,
       u.id, COALESCE(u.name, ''), COALESCE(u.status, ''), u.deleted_at IS NOT NULL
FROM churches c
LEFT JOIN users u ON lower(btrim(u.church_name)) = c.name_key
     AND u.role_id IN (SELECT id FROM roles WHERE name = $1)
ORDER BY lower(c.name), c.id, u.id`

// Churches reads every church with its church accounts (the user role, deleted
// ones flagged), one row per account.
func (r *store) Churches(ctx context.Context) ([]stats.ChurchRecord, error) {
	rows, err := r.db.Querier(ctx).Query(ctx, churchesQuery, accesscontrol.DefaultRoleName)
	if err != nil {
		return nil, fmt.Errorf("query churches: %w", err)
	}
	defer rows.Close()
	var out []stats.ChurchRecord
	for rows.Next() {
		var c stats.ChurchRecord
		var userID *int64
		var a stats.ChurchAccountInfo
		if err := rows.Scan(&c.ID, &c.Name, &c.StartMonth, &c.EndMonth, &userID, &a.Name, &a.Status, &a.Deleted); err != nil {
			return nil, fmt.Errorf("scan church: %w", err)
		}
		if n := len(out); n == 0 || out[n-1].ID != c.ID {
			out = append(out, c)
		}
		if userID != nil {
			a.UserID = *userID
			last := &out[len(out)-1]
			last.Accounts = append(last.Accounts, a)
		}
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("churches: %w", err)
	}
	return out, nil
}

// A report belongs to the church named on it when filed, else its author's church.
const analyticsReportsQuery = `
SELECT r.id, COALESCE(c.id, 0), COALESCE(c.name, btrim(COALESCE(r.church_name, u.church_name, ''))),
       COALESCE(r.worker, ''), r.year::int, EXTRACT(MONTH FROM to_date(r.month, 'Month'))::int,
       r.status, r.created_at, r.updated_at, rm.metric, rm.week_no::int, rm.value::double precision
FROM reports r
JOIN users u ON u.id = r.created_by
LEFT JOIN churches c ON c.name_key = lower(btrim(COALESCE(NULLIF(btrim(r.church_name), ''), u.church_name)))
LEFT JOIN report_metrics rm ON rm.report_id = r.id
WHERE r.deleted_at IS NULL AND r.year = ANY($1)
  AND EXTRACT(MONTH FROM to_date(r.month, 'Month')) BETWEEN $2 AND $3
ORDER BY r.id`

// AnalyticsReports reads the reports and their weekly values; Canonical is set by the caller.
func (r *store) AnalyticsReports(ctx context.Context, years []int, startMonth, endMonth int) ([]stats.ReportData, error) {
	rows, err := r.db.Querier(ctx).Query(ctx, analyticsReportsQuery, years, startMonth, endMonth)
	if err != nil {
		return nil, fmt.Errorf("query analytics reports: %w", err)
	}
	defer rows.Close()
	var out []stats.ReportData
	for rows.Next() {
		var d stats.ReportData
		var metric *string
		var week *int
		var value *float64
		if err := rows.Scan(&d.ID, &d.ChurchID, &d.ChurchName, &d.Worker, &d.Year, &d.Month, &d.Status,
			&d.CreatedAt, &d.UpdatedAt, &metric, &week, &value); err != nil {
			return nil, fmt.Errorf("scan analytics report: %w", err)
		}
		if n := len(out); n == 0 || out[n-1].ID != d.ID {
			d.Metrics = map[string]*[5]*float64{}
			out = append(out, d)
		}
		if metric != nil && week != nil && *week >= 1 && *week <= 5 {
			m := out[len(out)-1].Metrics
			if m[*metric] == nil {
				m[*metric] = new([5]*float64)
			}
			m[*metric][*week-1] = value
		}
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("analytics reports: %w", err)
	}
	return out, nil
}

// ReportingDefault reads the one reporting_settings row; nil when there is none.
func (r *store) ReportingDefault(ctx context.Context) (*time.Time, error) {
	var start time.Time
	err := r.db.Querier(ctx).QueryRow(ctx, "SELECT default_start_month FROM reporting_settings").Scan(&start)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("reporting default: %w", err)
	}
	return &start, nil
}

// SetChurchReporting updates one church's months under a row lock; run it in a
// transaction so the audit row commits with it.
func (r *store) SetChurchReporting(ctx context.Context, id int64, start, end *time.Time) (stats.ChurchRecord, stats.ChurchRecord, error) {
	q := r.db.Querier(ctx)
	before := stats.ChurchRecord{ID: id}
	err := q.QueryRow(ctx, "SELECT name, reporting_start_month, reporting_end_month FROM churches WHERE id = $1 FOR UPDATE", id).
		Scan(&before.Name, &before.StartMonth, &before.EndMonth)
	if errors.Is(err, pgx.ErrNoRows) {
		return before, before, stats.ErrChurchNotFound
	}
	if err != nil {
		return before, before, fmt.Errorf("get church: %w", err)
	}
	if _, err := q.Exec(ctx, "UPDATE churches SET reporting_start_month = $2, reporting_end_month = $3, updated_at = NOW() WHERE id = $1", id, start, end); err != nil {
		return before, before, fmt.Errorf("update church: %w", err)
	}
	after := before
	after.StartMonth, after.EndMonth = start, end
	return before, after, nil
}
