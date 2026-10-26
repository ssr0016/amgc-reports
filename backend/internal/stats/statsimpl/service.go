package statsimpl

import (
	"context"
	"fmt"
	"time"

	"github.com/ssr0016/pastoral-reports/backend/internal/report"
	"github.com/ssr0016/pastoral-reports/backend/internal/stats"
)

// service holds the rule for which month the admin cards describe: the
// previous month in Philippine time (D-A7, D-F8) unless the caller names one.
type service struct {
	store stats.Store
	now   func() time.Time
	loc   *time.Location // months are counted in Philippine time, whatever the server clock says
}

// NewService returns the stats.Service over store.
func NewService(store stats.Store) stats.Service { return newService(store) }

// newService is NewService with the concrete type, so tests can set the clock.
func newService(store stats.Store) *service {
	loc, err := time.LoadLocation("Asia/Manila")
	if err != nil {
		panic(fmt.Sprintf("load Asia/Manila (is tzdata installed?): %v", err))
	}
	return &service{store: store, now: time.Now, loc: loc}
}

// Get returns the counts for the given month and year. Both nil means the
// previous month. The month's canonical name is queried and returned.
func (s *service) Get(ctx context.Context, q stats.GetStatsQuery) (stats.AdminStats, error) {
	m, y, err := s.period(q)
	if err != nil {
		return stats.AdminStats{}, err
	}
	out, err := s.store.Counts(ctx, m, y)
	if err != nil {
		return stats.AdminStats{}, fmt.Errorf("stats for %s %d: %w", m, y, err)
	}
	return out, nil
}

// Tracker returns who has reported for the period, under the same rules as Get.
func (s *service) Tracker(ctx context.Context, q stats.GetStatsQuery) (stats.Tracker, error) {
	m, y, err := s.period(q)
	if err != nil {
		return stats.Tracker{}, err
	}
	rows, err := s.store.Tracker(ctx, m, y)
	if err != nil {
		return stats.Tracker{}, fmt.Errorf("tracker for %s %d: %w", m, y, err)
	}
	return stats.Tracker{Month: m, Year: y, Rows: rows}, nil
}

func (s *service) AnalyticsAttendance(ctx context.Context, q stats.AttendanceQuery) (stats.AttendanceAnalytics, error) {
	if q.Year <= 0 || !validAnalyticsMetric(q.Metric) {
		return stats.AttendanceAnalytics{}, stats.ErrAnalyticsQuery
	}
	if q.Status == "" {
		q.Status = stats.AttendanceStatusApproved
	}
	if q.Status != stats.AttendanceStatusApproved && q.Status != stats.AttendanceStatusAll {
		return stats.AttendanceAnalytics{}, stats.ErrAnalyticsQuery
	}
	out, err := s.store.AnalyticsAttendance(ctx, q)
	if err != nil {
		return stats.AttendanceAnalytics{}, fmt.Errorf("attendance analytics for %d: %w", q.Year, err)
	}
	out.Year, out.Metric, out.Status = q.Year, q.Metric, q.Status
	return out, nil
}

func validAnalyticsMetric(metric string) bool {
	for _, key := range report.MetricKeys {
		if metric == key {
			return true
		}
	}
	return false
}

// period resolves q to a canonical month name and a year: both nil means the
// previous month in Manila; only one, or an unknown month, is ErrPeriod.
func (s *service) period(q stats.GetStatsQuery) (string, int, error) {
	if (q.Month == nil) != (q.Year == nil) {
		return "", 0, stats.ErrPeriod
	}
	if q.Month == nil {
		t := s.now().In(s.loc)
		prev := time.Date(t.Year(), t.Month(), 1, 0, 0, 0, 0, s.loc).AddDate(0, -1, 0)
		return report.Months[prev.Month()-1], prev.Year(), nil
	}
	parsed, ok := report.ParseMonth(*q.Month)
	if !ok {
		return "", 0, stats.ErrPeriod
	}
	return report.Months[parsed-1], *q.Year, nil
}
