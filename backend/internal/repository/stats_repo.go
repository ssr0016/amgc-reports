package repository

import (
	"context"
	"fmt"

	sq "github.com/Masterminds/squirrel"

	"github.com/ssr0016/pastoral-reports/backend/internal/database"
	"github.com/ssr0016/pastoral-reports/backend/internal/model"
)

type StatsRepo struct{ db *database.DB }

func NewStatsRepo(db *database.DB) *StatsRepo { return &StatsRepo{db: db} }

// Counts returns the admin dashboard numbers for one month in a single read.
// Soft-deleted reports are not counted. The month matches case-insensitively, as
// in ReportRepo.List; month and year are echoed back as given.
func (r *StatsRepo) Counts(ctx context.Context, month string, year int) (model.AdminStats, error) {
	inPeriod := "FROM reports WHERE deleted_at IS NULL AND year = ? AND lower(month) = lower(?)"
	query, args, err := r.db.Builder.Select().
		Column(sq.Expr("(SELECT COUNT(*) "+inPeriod+")", year, month)).
		Column(sq.Expr("(SELECT COUNT(*) "+inPeriod+" AND completed)", year, month)).
		Column("(SELECT COUNT(*) FROM users)").
		Column(sq.Expr("(SELECT COUNT(*) FROM users u JOIN roles ro ON ro.id = u.role_id WHERE ro.name = ?)", "admin")).
		ToSql()
	if err != nil {
		return model.AdminStats{}, fmt.Errorf("build stats query: %w", err)
	}
	s := model.AdminStats{Month: month, Year: year}
	if err := r.db.Pool.QueryRow(ctx, query, args...).Scan(&s.ReportsTotal, &s.ReportsApproved, &s.UsersTotal, &s.AdminsTotal); err != nil {
		return model.AdminStats{}, fmt.Errorf("count stats: %w", err)
	}
	s.ReportsPending = s.ReportsTotal - s.ReportsApproved
	return s, nil
}
