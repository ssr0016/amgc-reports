package statsimpl

import (
	"cmp"
	"context"
	"errors"
	"fmt"
	"strconv"
	"time"

	"github.com/ssr0016/pastoral-reports/backend/internal/audit"
	"github.com/ssr0016/pastoral-reports/backend/internal/auth"
	"github.com/ssr0016/pastoral-reports/backend/internal/database"
	"github.com/ssr0016/pastoral-reports/backend/internal/user"

	"github.com/ssr0016/pastoral-reports/backend/internal/report"
	"github.com/ssr0016/pastoral-reports/backend/internal/stats"
)

// service holds the rule for which month the admin cards describe: the
// previous month in Philippine time (D-A7, D-F8) unless the caller names one.
type service struct {
	store stats.Store
	audit audit.Store
	tx    database.TxRunner
	now   func() time.Time
	loc   *time.Location // months are counted in Philippine time, whatever the server clock says
}

// NewService returns the stats.Service over store; church changes are audited
// to auditStore inside tx.
func NewService(store stats.Store, auditStore audit.Store, tx database.TxRunner) stats.Service {
	s := newService(store)
	s.audit, s.tx = auditStore, tx
	return s
}

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
	if q.Year < 2000 || q.Year > 2100 || !validAnalyticsMetric(q.Metric) {
		return stats.AttendanceAnalytics{}, stats.ErrAnalyticsQuery
	}
	if q.Period == "" {
		q.Period = stats.AttendancePeriodFullYear
	}
	start, end, ok := stats.AnalyticsPeriodBounds(q.Period)
	if !ok {
		return stats.AttendanceAnalytics{}, stats.ErrAnalyticsQuery
	}
	if q.Status == "" {
		q.Status = stats.AttendanceStatusApproved
	}
	if q.Status != stats.AttendanceStatusApproved && q.Status != stats.AttendanceStatusAll {
		return stats.AttendanceAnalytics{}, stats.ErrAnalyticsQuery
	}
	churches, err := s.store.Churches(ctx)
	if err != nil {
		return stats.AttendanceAnalytics{}, fmt.Errorf("churches: %w", err)
	}
	years := []int{q.Year}
	if q.Compare {
		years = append(years, q.Year-1)
	}
	reports, err := s.store.AnalyticsReports(ctx, years, start, end)
	if err != nil {
		return stats.AttendanceAnalytics{}, fmt.Errorf("attendance analytics for %d: %w", q.Year, err)
	}
	defaultStart, err := s.store.ReportingDefault(ctx)
	if err != nil {
		return stats.AttendanceAnalytics{}, err
	}
	return analyze(analysis{q: q, now: s.now().In(s.loc), churches: churches, reports: reports, defaultStart: defaultStart}), nil
}

// AnalyticsExcel builds the workbook from AnalyticsAttendance's answer.
func (s *service) AnalyticsExcel(ctx context.Context, q stats.AttendanceQuery) ([]byte, error) {
	data, err := s.AnalyticsAttendance(ctx, q)
	if err != nil {
		return nil, err
	}
	return buildAnalyticsWorkbook(data)
}

// AnalyticsCSV is one flat table from AnalyticsAttendance's answer.
func (s *service) AnalyticsCSV(ctx context.Context, q stats.AttendanceQuery, kind string) ([]byte, error) {
	if kind != stats.CSVWeeklyDetail && kind != stats.CSVCompliance {
		return nil, stats.ErrAnalyticsQuery
	}
	data, err := s.AnalyticsAttendance(ctx, q)
	if err != nil {
		return nil, err
	}
	return analyticsCSV(data, kind)
}

// SetChurchReporting stores a church's reporting months (activity months) and
// audits UPDATE_CHURCH_REPORTING with the months before and after, in one transaction.
func (s *service) SetChurchReporting(ctx context.Context, actorID int64, ip string, churchID int64, req stats.ChurchReportingRequest) (stats.ChurchScope, error) {
	start, err := monthStart(req.StartMonth, time.UTC)
	if err != nil {
		return stats.ChurchScope{}, stats.ErrChurchPeriod
	}
	end, err := monthStart(req.EndMonth, time.UTC)
	if err != nil {
		return stats.ChurchScope{}, stats.ErrChurchPeriod
	}
	defaultStart, err := s.store.ReportingDefault(ctx)
	if err != nil {
		return stats.ChurchScope{}, err
	}
	// The end is checked against the start the church will have: its own, else the default.
	if effective := cmp.Or(start, defaultStart); end != nil && effective != nil && end.Before(*effective) {
		return stats.ChurchScope{}, stats.ErrChurchPeriod
	}
	caller, ok := user.CallerFrom(ctx, actorID)
	if !ok {
		return stats.ChurchScope{}, auth.ErrUnauthorized
	}
	var after stats.ChurchRecord
	err = s.tx.InTransaction(ctx, func(ctx context.Context) error {
		before, a, err := s.store.SetChurchReporting(ctx, churchID, start, end)
		if err != nil {
			return err
		}
		after = a
		e := user.AuditLog(caller.User, ip, audit.ActionUpdateChurchReporting, &user.User{}, map[string]any{
			"from": map[string]string{"start_month": monthText(before.StartMonth), "end_month": monthText(before.EndMonth)},
			"to":   map[string]string{"start_month": req.StartMonth, "end_month": req.EndMonth},
		})
		id := strconv.FormatInt(churchID, 10)
		e.TargetType, e.TargetID, e.TargetName = audit.TargetTypeChurch, &id, &before.Name
		return s.audit.Insert(ctx, e)
	})
	if err != nil {
		if errors.Is(err, stats.ErrChurchNotFound) {
			return stats.ChurchScope{}, err
		}
		return stats.ChurchScope{}, fmt.Errorf("set church reporting: %w", err)
	}
	effective := cmp.Or(after.StartMonth, defaultStart)
	return stats.ChurchScope{ID: churchID, Name: after.Name, StartMonth: monthText(effective), StartIsDefault: after.StartMonth == nil && effective != nil,
		EndMonth: monthText(after.EndMonth), NeedsReview: effective == nil}, nil
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
