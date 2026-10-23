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
		Column(sq.Expr("(SELECT COUNT(*) "+inPeriod+" AND completed)", year, stored)).
		Column("(SELECT COUNT(*) FROM users WHERE deleted_at IS NULL)").
		Column(sq.Expr("(SELECT COUNT(*) FROM users u JOIN roles ro ON ro.id = u.role_id WHERE u.deleted_at IS NULL AND ro.name = ?)", "admin")).
		ToSql()
	if err != nil {
		return stats.AdminStats{}, fmt.Errorf("build stats query: %w", err)
	}
	s := stats.AdminStats{Month: month, Year: year}
	if err := r.db.Querier(ctx).QueryRow(ctx, query, args...).Scan(&s.ReportsTotal, &s.ReportsApproved, &s.UsersTotal, &s.AdminsTotal); err != nil {
		return stats.AdminStats{}, fmt.Errorf("count stats: %w", err)
	}
	s.ReportsPending = s.ReportsTotal - s.ReportsApproved
	return s, nil
}

// Tracker reads each church account (report.ChurchAccount) and its one active report for the month
// (a unique index allows one per account and period) in a single query (D-A3).
func (r *store) Tracker(ctx context.Context, month string, year int) ([]stats.TrackerRow, error) {
	query, args, err := r.db.Builder.
		Select("u.id", "u.name", "u.email", "COALESCE(u.church_name, '')", "u.status", "r.completed").
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
		var completed *bool // NULL when there is no report this month
		if err := rows.Scan(&row.UserID, &row.Name, &row.Email, &row.ChurchName, &row.UserStatus, &completed); err != nil {
			return nil, fmt.Errorf("scan tracker: %w", err)
		}
		switch {
		case completed == nil:
			row.Status = stats.TrackerNotSubmitted
		case *completed:
			row.Status = stats.TrackerApproved
		default:
			row.Status = stats.TrackerForReview
		}
		out = append(out, row)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("tracker: %w", err)
	}
	return out, nil
}
