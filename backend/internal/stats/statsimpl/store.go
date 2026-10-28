package statsimpl

import (
	"context"
	"fmt"
	"time"

	sq "github.com/Masterminds/squirrel"
	"golang.org/x/sync/errgroup"

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

// AnalyticsAttendance reads the year, and the year before when q.Compare, concurrently.
// Its reads run in parallel on separate pool connections, so it must not be called
// inside a transaction: a pgx.Tx is one connection and cannot run queries concurrently.
func (r *store) AnalyticsAttendance(ctx context.Context, q stats.AttendanceQuery) (stats.AttendanceAnalytics, error) {
	var current, previous stats.AttendanceAnalytics
	g, gctx := errgroup.WithContext(ctx)
	g.Go(func() (err error) {
		current, err = r.analyticsAttendanceYear(gctx, q, q.Year)
		return err
	})
	if q.Compare {
		g.Go(func() (err error) {
			previous, err = r.analyticsAttendanceYear(gctx, q, q.Year-1)
			return err
		})
	}
	if err := g.Wait(); err != nil {
		return stats.AttendanceAnalytics{}, err
	}
	if q.Compare {
		current.Comparison = &stats.AttendanceComparison{Year: q.Year - 1, Trend: previous.Trend, Churches: previous.Churches, Detail: previous.Detail,
			Weeks: previous.Weeks, WeeksByMonth: previous.WeeksByMonth}
	}
	return current, nil
}

const attendancePeriodWhere = "AND EXTRACT(MONTH FROM to_date(r.month, 'Month')) BETWEEN ? AND ?"

func (r *store) analyticsAttendanceYear(ctx context.Context, q stats.AttendanceQuery, year int) (stats.AttendanceAnalytics, error) {
	statusWhere := "AND r.status = 'approved'"
	if q.Status == stats.AttendanceStatusAll {
		statusWhere = "AND r.status IN ('pending', 'needs_revision', 'approved')"
	}
	startMonth, endMonth, _ := stats.AnalyticsPeriodBounds(q.Period)

	var (
		values   map[[2]int]float64
		churches []stats.AttendanceChurch
		summary  []stats.AttendanceMetricSummary
		detail   []stats.AttendanceDetail
		noReport []string
		weeks    map[int]int
	)
	g, gctx := errgroup.WithContext(ctx)
	g.Go(func() (err error) {
		values, err = r.attendanceTrend(gctx, q.Metric, year, startMonth, endMonth, statusWhere)
		return err
	})
	g.Go(func() (err error) {
		churches, err = r.attendanceChurches(gctx, q.Metric, year, startMonth, endMonth, statusWhere)
		return err
	})
	g.Go(func() (err error) {
		summary, detail, err = r.attendanceSummary(gctx, year, startMonth, endMonth, statusWhere)
		return err
	})
	g.Go(func() (err error) {
		noReport, err = r.noReportChurches(gctx, year, startMonth, endMonth, statusWhere)
		return err
	})
	g.Go(func() (err error) {
		weeks, err = r.reportedWeeks(gctx, year, startMonth, endMonth, statusWhere)
		return err
	})
	if err := g.Wait(); err != nil {
		return stats.AttendanceAnalytics{}, err
	}

	trend := make([]stats.AttendanceTrendPoint, 0, (endMonth-startMonth+1)*5)
	for month := startMonth; month <= endMonth; month++ {
		name := report.Months[month-1]
		for week := 1; week <= 5; week++ {
			trend = append(trend, stats.AttendanceTrendPoint{Month: name, Week: week, Value: values[[2]int{month, week}]})
		}
	}
	return stats.AttendanceAnalytics{Year: year, Metric: q.Metric, Status: q.Status, Period: q.Period, Trend: trend, Churches: churches, Summary: summary, Detail: detail, NoReportChurches: noReport, Weeks: totalWeeks(weeks), WeeksByMonth: weeks}, nil
}

func totalWeeks(byMonth map[int]int) int {
	total := 0
	for _, n := range byMonth {
		total += n
	}
	return total
}

// reportedWeeks counts, per month number, the distinct report weeks that hold any
// metric value from a report matching statusWhere: the weeks with reports.
func (r *store) reportedWeeks(ctx context.Context, year, startMonth, endMonth int, statusWhere string) (map[int]int, error) {
	query, args, err := r.db.Builder.
		Select("EXTRACT(MONTH FROM to_date(r.month, 'Month'))::int", "COUNT(DISTINCT rm.week_no)::int").
		From("reports r").
		Join("report_metrics rm ON rm.report_id = r.id").
		Where("r.deleted_at IS NULL AND r.year = ? "+attendancePeriodWhere+" "+statusWhere, year, startMonth, endMonth).
		GroupBy("1").
		ToSql()
	if err != nil {
		return nil, fmt.Errorf("build reported weeks query: %w", err)
	}
	rows, err := r.db.Querier(ctx).Query(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("query reported weeks: %w", err)
	}
	defer rows.Close()
	weeks := make(map[int]int)
	for rows.Next() {
		var month, n int
		if err := rows.Scan(&month, &n); err != nil {
			return nil, fmt.Errorf("scan reported weeks: %w", err)
		}
		weeks[month] = n
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("reported weeks: %w", err)
	}
	return weeks, nil
}

// noReportChurches lists the churches of active church accounts (as counted by
// Counts' churches total) that have no report matching statusWhere in the period.
// An account's own reports count as its church's, as in Tracker.
func (r *store) noReportChurches(ctx context.Context, year, startMonth, endMonth int, statusWhere string) ([]string, error) {
	query, args, err := r.db.Builder.
		Select("MIN(btrim(u.church_name))").
		From("users u").
		Join("roles ro ON ro.id = u.role_id").
		Where("ro.name = 'user' AND u.deleted_at IS NULL AND u.status = 'active' AND btrim(COALESCE(u.church_name, '')) <> ''").
		GroupBy("lower(btrim(u.church_name))").
		// A church has reported when any of its accounts filed, or any report carries its name.
		Having(sq.Expr("NOT bool_or(EXISTS (SELECT 1 FROM reports r WHERE (r.created_by = u.id OR lower(btrim(r.church_name)) = lower(btrim(u.church_name))) AND r.deleted_at IS NULL AND r.year = ? "+attendancePeriodWhere+" "+statusWhere+"))", year, startMonth, endMonth)).
		OrderBy("1").
		ToSql()
	if err != nil {
		return nil, fmt.Errorf("build no-report churches query: %w", err)
	}
	rows, err := r.db.Querier(ctx).Query(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("query no-report churches: %w", err)
	}
	defer rows.Close()
	churches := make([]string, 0)
	for rows.Next() {
		var church string
		if err := rows.Scan(&church); err != nil {
			return nil, fmt.Errorf("scan no-report church: %w", err)
		}
		churches = append(churches, church)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("no-report churches: %w", err)
	}
	return churches, nil
}

// attendanceTrend sums metric per (month, week) for the period.
func (r *store) attendanceTrend(ctx context.Context, metric string, year, startMonth, endMonth int, statusWhere string) (map[[2]int]float64, error) {
	query, args, err := r.db.Builder.
		Select("EXTRACT(MONTH FROM to_date(r.month, 'Month'))::int", "rm.week_no", "SUM(rm.value)::double precision").
		From("reports r").
		Join("report_metrics rm ON rm.report_id = r.id").
		Where("r.deleted_at IS NULL AND r.year = ? AND rm.metric = ? "+attendancePeriodWhere+" "+statusWhere, year, metric, startMonth, endMonth).
		GroupBy("1", "2").
		OrderBy("1", "2").
		ToSql()
	if err != nil {
		return nil, fmt.Errorf("build attendance trend query: %w", err)
	}
	rows, err := r.db.Querier(ctx).Query(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("query attendance trend: %w", err)
	}
	defer rows.Close()
	values := make(map[[2]int]float64)
	for rows.Next() {
		var month, week int
		var value float64
		if err := rows.Scan(&month, &week, &value); err != nil {
			return nil, fmt.Errorf("scan attendance trend: %w", err)
		}
		values[[2]int{month, week}] = value
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("attendance trend: %w", err)
	}
	return values, nil
}

// attendanceChurches sums metric per church for the period.
func (r *store) attendanceChurches(ctx context.Context, metric string, year, startMonth, endMonth int, statusWhere string) ([]stats.AttendanceChurch, error) {
	query, args, err := r.db.Builder.
		Select("COALESCE(NULLIF(btrim(r.church_name), ''), 'Unassigned')", "SUM(rm.value)::double precision").
		From("reports r").
		Join("report_metrics rm ON rm.report_id = r.id").
		Where("r.deleted_at IS NULL AND r.year = ? AND rm.metric = ? "+attendancePeriodWhere+" "+statusWhere, year, metric, startMonth, endMonth).
		GroupBy("1").
		OrderBy("1").
		ToSql()
	if err != nil {
		return nil, fmt.Errorf("build attendance church query: %w", err)
	}
	rows, err := r.db.Querier(ctx).Query(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("query attendance churches: %w", err)
	}
	defer rows.Close()
	churches := []stats.AttendanceChurch{}
	for rows.Next() {
		var church stats.AttendanceChurch
		if err := rows.Scan(&church.ChurchName, &church.Value); err != nil {
			return nil, fmt.Errorf("scan attendance church: %w", err)
		}
		churches = append(churches, church)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("attendance churches: %w", err)
	}
	return churches, nil
}

func (r *store) attendanceSummary(ctx context.Context, year, startMonth, endMonth int, statusWhere string) ([]stats.AttendanceMetricSummary, []stats.AttendanceDetail, error) {
	query, args, err := r.db.Builder.
		Select("rm.metric", "COALESCE(NULLIF(btrim(r.church_name), ''), 'Unassigned')",
			"EXTRACT(MONTH FROM to_date(r.month, 'Month'))::int", "SUM(rm.value)::double precision").
		From("reports r").
		Join("report_metrics rm ON rm.report_id = r.id").
		Where("r.deleted_at IS NULL AND r.year = ? AND EXTRACT(MONTH FROM to_date(r.month, 'Month')) BETWEEN ? AND ? "+statusWhere, year, startMonth, endMonth).
		GroupBy("1", "2", "3").
		OrderBy("1", "2", "3").
		ToSql()
	if err != nil {
		return nil, nil, fmt.Errorf("build attendance summary query: %w", err)
	}
	rows, err := r.db.Querier(ctx).Query(ctx, query, args...)
	if err != nil {
		return nil, nil, fmt.Errorf("query attendance summary: %w", err)
	}
	defer rows.Close()

	valuesByMetric := make(map[string]map[string]float64)
	monthlyByMetric := make(map[string]*[13]float64) // index 1-12 is the month
	var detail []stats.AttendanceDetail
	churchNames := make([]string, 0)
	seenChurches := make(map[string]bool)
	for rows.Next() {
		var metric, church string
		var month int
		var value float64
		if err := rows.Scan(&metric, &church, &month, &value); err != nil {
			return nil, nil, fmt.Errorf("scan attendance summary: %w", err)
		}
		if valuesByMetric[metric] == nil {
			valuesByMetric[metric] = make(map[string]float64)
			monthlyByMetric[metric] = new([13]float64)
		}
		valuesByMetric[metric][church] += value
		detail = append(detail, stats.AttendanceDetail{Metric: metric, ChurchName: church, Month: month, Value: value})
		monthlyByMetric[metric][month] += value
		if !seenChurches[church] {
			seenChurches[church] = true
			churchNames = append(churchNames, church)
		}
	}
	if err := rows.Err(); err != nil {
		return nil, nil, fmt.Errorf("attendance summary: %w", err)
	}

	summary := make([]stats.AttendanceMetricSummary, 0, len(report.MetricKeys))
	for _, group := range stats.MetricGroups {
		for _, metric := range group.Metrics {
			metricValues := valuesByMetric[metric]
			var total *float64
			if len(metricValues) > 0 {
				sum := 0.0
				for _, value := range metricValues {
					sum += value
				}
				total = &sum
			}
			metricChurches := make([]stats.AttendanceSummaryChurch, 0, len(churchNames))
			for _, church := range churchNames {
				value, ok := metricValues[church]
				if ok {
					metricChurches = append(metricChurches, stats.AttendanceSummaryChurch{ChurchName: church, Value: &value})
				} else {
					metricChurches = append(metricChurches, stats.AttendanceSummaryChurch{ChurchName: church})
				}
			}
			peak, peakValue := peakMonth(monthlyByMetric[metric])
			summary = append(summary, stats.AttendanceMetricSummary{Metric: metric, Group: group.Name, Total: total, PeakMonth: peak, PeakValue: peakValue, Churches: metricChurches})
		}
	}
	return summary, detail, nil
}

// peakMonth is the month with the highest total, earliest on a tie; "" when
// none is above 0. monthly is indexed by month number, 1-12.
func peakMonth(monthly *[13]float64) (string, float64) {
	peak, peakValue := "", 0.0
	if monthly == nil {
		return peak, peakValue
	}
	for m := 1; m <= 12; m++ {
		if monthly[m] > peakValue {
			peak, peakValue = time.Month(m).String(), monthly[m]
		}
	}
	return peak, peakValue
}
