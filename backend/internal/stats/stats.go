// Package stats is the admin panel's counts and the Analytics page: reports for
// one month, the account totals, and the period analytics with their exports.
// Callers depend on the interfaces here; statsimpl implements them and rest serves them.
package stats

import (
	"context"
	"time"
)

// Service answers GET /admin/stats and the analytics endpoints.
type Service interface {
	// Get returns the counts for the period in q; an empty q means the previous
	// month in Asia/Manila. ErrPeriod when the period is malformed.
	Get(ctx context.Context, q GetStatsQuery) (AdminStats, error)
	// Tracker returns who has reported for the period in q, with the same
	// period rules and ErrPeriod as Get (D-A3).
	Tracker(ctx context.Context, q GetStatsQuery) (Tracker, error)
	// AnalyticsAttendance is the Analytics page; ErrAnalyticsQuery when q is invalid.
	AnalyticsAttendance(ctx context.Context, q AttendanceQuery) (AttendanceAnalytics, error)
	// AnalyticsExcel is AnalyticsAttendance as the page's .xlsx download.
	AnalyticsExcel(ctx context.Context, q AttendanceQuery) ([]byte, error)
	// AnalyticsCSV is one flat table of it: CSVWeeklyDetail or CSVCompliance.
	AnalyticsCSV(ctx context.Context, q AttendanceQuery, kind string) ([]byte, error)
	// SetChurchReporting sets a church's reporting months and audits the change.
	SetChurchReporting(ctx context.Context, actorID int64, ip string, churchID int64, req ChurchReportingRequest) (ChurchScope, error)
}

// The CSV exports.
const (
	CSVWeeklyDetail = "weekly_detail"
	CSVCompliance   = "compliance"
)

// Store is the read side.
type Store interface {
	Counts(ctx context.Context, month string, year int) (AdminStats, error)
	// Tracker lists every church account (report.ChurchAccount), whatever its
	// status, with its report status for the month, by church then name.
	Tracker(ctx context.Context, month string, year int) ([]TrackerRow, error)
	// Churches lists every church with its church accounts, by name.
	Churches(ctx context.Context) ([]ChurchRecord, error)
	// AnalyticsReports reads every report of the years for months start..end,
	// whatever its status, with its weekly values and its church.
	AnalyticsReports(ctx context.Context, years []int, startMonth, endMonth int) ([]ReportData, error)
	// ReportingDefault is the organization-wide start month; nil when not configured.
	ReportingDefault(ctx context.Context) (*time.Time, error)
	// SetChurchReporting stores a church's months and returns it before and
	// after; ErrChurchNotFound when there is no such church.
	SetChurchReporting(ctx context.Context, id int64, start, end *time.Time) (before, after ChurchRecord, err error)
}
