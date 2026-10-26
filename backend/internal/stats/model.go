package stats

import "errors"

// ErrPeriod is returned when only one of month and year is given, or the month
// is not an English month name. The handler maps it to 422.
var ErrPeriod = errors.New("month and year must be given together, and the month must be a month name")

var ErrAnalyticsQuery = errors.New("year, metric, and status are invalid")

const (
	AttendanceStatusApproved = "approved"
	AttendanceStatusAll      = "all"
)

type AttendanceQuery struct {
	Year   int
	Metric string
	Status string
}

type AttendanceAnalytics struct {
	Year     int                    `json:"year"`
	Metric   string                 `json:"metric"`
	Status   string                 `json:"status"`
	Trend    []AttendanceTrendPoint `json:"trend"`
	Churches []AttendanceChurch     `json:"churches"`
}

type AttendanceTrendPoint struct {
	Month string  `json:"month"`
	Week  int     `json:"week"`
	Value float64 `json:"value"`
}

type AttendanceChurch struct {
	ChurchName string  `json:"church_name"`
	Value      float64 `json:"value"`
}

// GetStatsQuery is the period the caller asks for. Both nil means the previous
// month; setting only one is ErrPeriod.
type GetStatsQuery struct {
	Month *string
	Year  *int
}

// AdminStats is the body of GET /admin/stats: counts for one month, plus the
// account totals, which do not depend on the month. Counts only, no report text.
//
// The swagger name is pinned to the one it had in package model, so the generated
// frontend types (and D6, wire names) do not change when the package moves.
type AdminStats struct {
	Month                string `json:"month"`
	Year                 int    `json:"year"`
	ReportsTotal         int64  `json:"reports_total"`
	ReportsApproved      int64  `json:"reports_approved"`
	ReportsPending       int64  `json:"reports_pending"`
	ReportsNeedsRevision int64  `json:"reports_needs_revision"`
	UsersTotal           int64  `json:"users_total"`
	AdminsTotal          int64  `json:"admins_total"`
	ExpectedReports      int64  `json:"expected_reports"`
	NoReport             int64  `json:"no_report"`
	ChurchesTotal        int64  `json:"churches_total"`
} // @name github_com_ssr0016_pastoral-reports_backend_internal_model.AdminStats

// The Tracker statuses (D-A3): an account's report for the month is approved,
// filed and waiting for review, or missing.
const (
	TrackerSubmitted     = "submitted"
	TrackerNoReport      = "no_report"
	TrackerApproved      = "approved"
	TrackerPending       = "pending"
	TrackerNeedsRevision = "needs_revision"
)

// Tracker is the body of GET /admin/tracker: every account expected to report
// (a church account, D-A24) and its report status for one month. No report text.
type Tracker struct {
	Month string       `json:"month"`
	Year  int          `json:"year"`
	Rows  []TrackerRow `json:"rows"`
} // @name github_com_ssr0016_pastoral-reports_backend_internal_model.Tracker

// TrackerRow is one church account; ChurchName is never blank.
// UserStatus is the account status (active, inactive, disciplinary, deceased).
type TrackerRow struct {
	UserID           int64  `json:"user_id"`
	Name             string `json:"name"`
	Email            string `json:"email"`
	ChurchName       string `json:"church_name"`
	UserStatus       string `json:"user_status"`
	SubmissionStatus string `json:"submission_status" enums:"submitted,no_report"`
	ReviewStatus     string `json:"review_status,omitempty" enums:"approved,pending,needs_revision"`
} // @name github_com_ssr0016_pastoral-reports_backend_internal_model.TrackerRow
