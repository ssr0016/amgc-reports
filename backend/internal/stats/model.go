package stats

import (
	"errors"
	"fmt"
	"strings"
	"time"
)

// ErrPeriod is returned when only one of month and year is given, or the month
// is not an English month name. The handler maps it to 422.
var ErrPeriod = errors.New("month and year must be given together, and the month must be a month name")

var ErrAnalyticsQuery = errors.New("year, metric, status, period or churches are invalid")

// ErrChurchNotFound and ErrChurchPeriod are the church eligibility errors (404, 422).
var (
	ErrChurchNotFound = errors.New("church not found")
	ErrChurchPeriod   = errors.New("the reporting end month must not be before the church's start month")
)

const (
	AttendanceStatusApproved = "approved"
	AttendanceStatusAll      = "all"
	AttendancePeriodFullYear = "full_year"
)

// AttendanceQuery is the Analytics page's filter; every KPI, table, chart and
// export reads it the same way. No ChurchIDs means every church.
type AttendanceQuery struct {
	Year      int
	Metric    string
	Status    string
	Period    string
	Compare   bool
	ChurchIDs []int64
}

// Units say what a metric's values count (D-A31).
const (
	UnitPeople     = "people"      // attendance entries: a headcount per service, not unique persons
	UnitCurrency   = "php"         // pesos
	UnitCount      = "count"       // homes, groups, sermons, persons
	UnitAsReported = "as_reported" // activity values: participants or events, not established
)

// MetricGroup is a heading the analytics summary groups metrics under.
type MetricGroup struct {
	Name    string
	Unit    string
	Metrics []string
}

// MetricGroups orders every report.MetricKeys entry under one heading. Only the
// Attendance group gets per-service averages.
var MetricGroups = []MetricGroup{
	{"Attendance", UnitPeople, []string{"worship_service", "sunday_school", "prayer_meeting", "bible_studies", "mens_fellowship", "womens_fellowship", "youth_fellowship", "children_fellowship"}},
	{"Finance", UnitCurrency, []string{"tithes_offering"}},
	{"Pastoral Work", UnitCount, []string{"home_visited", "bible_study_group_led", "sermon_preached"}},
	{"Evangelism & Growth", UnitCount, []string{"person_newly_contacted", "person_followed_up", "person_led_to_christ", "baptism", "person_evangelized"}},
	{"Ministry & Events", UnitAsReported, []string{"outreach", "training", "leadership", "family_day", "other"}},
}

// MetricLabels are the labels every view and export uses (frontend METRICS).
var MetricLabels = map[string]string{
	"worship_service": "Worship Service", "sunday_school": "Sunday School", "prayer_meeting": "Prayer Meeting",
	"bible_studies": "Bible Studies", "mens_fellowship": "Men's Fellowship", "womens_fellowship": "Women's Fellowship",
	"youth_fellowship": "Youth Fellowship", "children_fellowship": "Children's Fellowship", "tithes_offering": "Tithes & Offering",
	"home_visited": "Homes Visited", "bible_study_group_led": "Bible Study Groups Led", "sermon_preached": "Sermons/Messages Preached",
	"person_newly_contacted": "Persons Newly Contacted", "person_followed_up": "Persons Followed Up",
	"person_led_to_christ": "Persons Led to Christ", "person_evangelized": "Legacy Persons Evangelized — meaning unverified",
	"outreach": "Outreach", "training": "Training/Seminars", "leadership": "Leadership (Conference/Training)", "baptism": "Baptisms",
	"other": "Other", "family_day": "Family Day",
}

// Month states of the reporting calendar, in Asia/Manila (D-A31): a month's
// report opens when the month ends, is due on the 15th of the next month, may be
// filed late until that month's last day, and closes after it.
const (
	MonthNotYetOpen = "not_yet_open" // the activity month has not ended
	MonthOpen       = "open"         // 1st-15th of the next month
	MonthGrace      = "grace"        // 16th to the end of the next month
	MonthClosed     = "closed"       // submission closed
)

// Compliance cell states for one church and activity month.
const (
	CellNotRequired = "not_required" // outside the church's reporting months
	CellNotReviewed = "not_reviewed" // no start month: neither the organization default nor an override is set
	CellInactive    = "inactive"     // no active church account and no end month: not expected
	CellNotYetOpen  = "not_yet_open"
	CellAwaiting    = "awaiting_submission" // missing, window open through the 15th
	CellOverdue     = "overdue_grace"       // missing after the 15th, still accepted
	CellNotFiled    = "not_filed"           // missing after the window closed
	CellSubmitted   = "submitted"
)

// Submission timeliness, from the report's creation time in Asia/Manila.
const (
	TimelyOnTime      = "on_time"      // by the 15th of the next month
	TimelyLate        = "late"         // in the grace period
	TimelyAfterCutoff = "after_cutoff" // entered after the window, by an admin (reports:write_any)
	TimelyBeforeOpen  = "before_open"  // entered before the month ended, by an admin
)

// AttendanceAnalytics is GET /admin/analytics/attendance. Values are computed
// once from the weekly report data and serve the page, Excel and CSV alike.
type AttendanceAnalytics struct {
	Year   int    `json:"year"`
	Metric string `json:"metric"`
	Status string `json:"status"`
	Period string `json:"period"`
	// GeneratedAt is when this answer was computed, in Asia/Manila.
	GeneratedAt time.Time `json:"generated_at"`
	// Months is every month of the period with its reporting state.
	Months []AnalyticsMonth `json:"months"`
	// DefaultStartMonth is the organization-wide first reporting month ("YYYY-MM"),
	// used by every church without its own start month; "" when not configured.
	DefaultStartMonth string `json:"default_start_month"`
	// CutoffMonth is the last month of the period that has ended (0 when none):
	// analytics count months 1..CutoffMonth only, so an unfinished year is year to date.
	CutoffMonth int `json:"cutoff_month"`
	// ChurchOptions is every church that could be filtered on.
	ChurchOptions []ChurchOption `json:"church_options"`
	// Churches is the churches in scope (the filter applied), each with its eligibility.
	Churches []ChurchScope `json:"churches"`
	// Coverage is report counts over the counted months, for the status filter.
	Coverage Coverage `json:"coverage"`
	// Metrics is every metric's period figures, in MetricGroups order.
	Metrics []MetricSummary `json:"metrics"`
	// Trend is the selected metric per month of the period.
	Trend []TrendPoint `json:"trend"`
	// Slots is the selected metric per month and reporting-week slot (Week 1-5 of the form, not calendar weeks).
	Slots []SlotPoint `json:"slots"`
	// ChurchRows is the selected metric for each church in scope, one row per church.
	ChurchRows []ChurchMetric `json:"church_rows"`
	// Compliance is the church-by-month submission matrix; it ignores the status filter.
	Compliance       []ComplianceRow  `json:"compliance"`
	ComplianceTotals ComplianceTotals `json:"compliance_totals"`
	Comparison       *Comparison      `json:"comparison,omitempty"`
	// Reports is every report read for the period and churches, for the weekly-detail exports.
	Reports []ReportData `json:"-"`
}

type AnalyticsMonth struct {
	Month int    `json:"month"`
	Name  string `json:"name"`
	State string `json:"state" enums:"not_yet_open,open,grace,closed"`
}

type ChurchOption struct {
	ID   int64  `json:"id"`
	Name string `json:"name"`
}

// ChurchScope is a church and its reporting eligibility. Months are "YYYY-MM".
// StartMonth is the effective start: the church's override, else the organization
// default (StartIsDefault). NeedsReview is true only when neither is set. Active is
// whether the church has an active church account (as the admin cards count it); an
// inactive church is listed only with reports or an end month, and without an end
// month it is not expected to report.
type ChurchScope struct {
	ID             int64  `json:"id"`
	Name           string `json:"name"`
	StartMonth     string `json:"start_month,omitempty"`
	StartIsDefault bool   `json:"start_is_default"`
	EndMonth       string `json:"end_month,omitempty"`
	NeedsReview    bool   `json:"needs_review"`
	Active         bool   `json:"active"`
	// AccountStatus is "active" when any church account is active, else the first
	// account's status, or "none" when no account remains.
	AccountStatus string `json:"account_status"`
	Workers       string `json:"workers"`
}

// Coverage counts church-month reports in scope (status filter applied) against
// the church-months expected, for the counted months.
type Coverage struct {
	Months         int `json:"months"`
	ReportsInScope int `json:"reports_in_scope"`
	// ExpectedReported counts the reports in scope for expected church-months; OutsideReports
	// the ones for months a church was not required to report (they still count in totals).
	ExpectedReported int `json:"expected_reported"`
	OutsideReports   int `json:"outside_reports"`
	ChurchesReported int `json:"churches_reported"`
	Expected         int `json:"expected"`
	Churches         int `json:"churches"`
}

// MetricSummary is one metric's figures over the counted months. Total is the
// sum of recorded values (nil when none); Observations counts recorded church-week
// values, explicit zeros included and blanks excluded.
type MetricSummary struct {
	Metric           string   `json:"metric"`
	Label            string   `json:"label"`
	Group            string   `json:"group"`
	Unit             string   `json:"unit"`
	Total            *float64 `json:"total" extensions:"x-nullable"`
	Observations     int      `json:"observations"`
	ZeroObservations int      `json:"zero_observations"`
	// ReportsWithValue counts church-month reports with at least one value; ChurchesWithValue the churches.
	ReportsWithValue  int `json:"reports_with_value"`
	ChurchesWithValue int `json:"churches_with_value"`
	// AvgPerChurchService is Total / Observations: attendance per reported church-service (Attendance only).
	AvgPerChurchService *float64 `json:"avg_per_church_service,omitempty" extensions:"x-nullable"`
	// NetworkWeekly is the average network total per reporting-week slot with any value (Attendance only).
	NetworkWeekly    *float64      `json:"network_weekly,omitempty" extensions:"x-nullable"`
	NetworkSlots     int           `json:"network_slots"`
	NetworkSlotsFull int           `json:"network_slots_full"`
	PeakMonth        string        `json:"peak_month"`
	PeakValue        float64       `json:"peak_value"`
	Churches         []ChurchValue `json:"churches"`
}

// ChurchValue is one church's period figure for a metric. State is "value"
// (Value set, 0 included), "blank" (reported, metric left blank), "not_approved"
// (reports filed but none approved, under the Approved filter) or "no_report".
type ChurchValue struct {
	ChurchID     int64    `json:"church_id"`
	Value        *float64 `json:"value" extensions:"x-nullable"`
	Observations int      `json:"observations"`
	State        string   `json:"state" enums:"value,blank,not_approved,no_report"`
}

// TrendPoint is the selected metric for one month. Value is nil when no church
// recorded it (a gap, never 0); Partial when fewer churches reported than expected.
type TrendPoint struct {
	Month             int      `json:"month"`
	Name              string   `json:"name"`
	State             string   `json:"state"`
	Counted           bool     `json:"counted"`
	Value             *float64 `json:"value" extensions:"x-nullable"`
	Observations      int      `json:"observations"`
	ChurchesWithValue int      `json:"churches_with_value"`
	ChurchesReported  int      `json:"churches_reported"`
	// ExpectedReported is the churches reported among the expected ones; Partial when below ChurchesExpected.
	ExpectedReported int  `json:"expected_reported"`
	ChurchesExpected int  `json:"churches_expected"`
	Partial          bool `json:"partial"`
}

// SlotPoint is the network total of the selected metric for one month and
// reporting-week slot, with the churches that recorded it.
type SlotPoint struct {
	Month             int      `json:"month"`
	Week              int      `json:"week"`
	Total             *float64 `json:"total" extensions:"x-nullable"`
	ChurchesWithValue int      `json:"churches_with_value"`
	ChurchesReported  int      `json:"churches_reported"`
}

// ChurchMetric is one church's row in the church comparison, for the selected metric.
type ChurchMetric struct {
	ChurchID     int64    `json:"church_id"`
	ChurchName   string   `json:"church_name"`
	Total        *float64 `json:"total" extensions:"x-nullable"`
	Observations int      `json:"observations"`
	// Average is Total / Observations (Attendance only).
	Average         *float64 `json:"average,omitempty" extensions:"x-nullable"`
	PreviousTotal   *float64 `json:"previous_total,omitempty" extensions:"x-nullable"`
	PreviousObs     int      `json:"previous_observations"`
	PreviousAverage *float64 `json:"previous_average,omitempty" extensions:"x-nullable"`
	// Change compares averages (Attendance) or totals; nil when either side has no data or the previous is 0.
	Change      *float64 `json:"change,omitempty" extensions:"x-nullable"`
	TithesTotal *float64 `json:"tithes_total" extensions:"x-nullable"`
	Submitted   int      `json:"submitted"`
	Approved    int      `json:"approved"`
	Expected    int      `json:"expected"`
	// Completeness is Submitted / Expected over the counted months; nil when nothing is expected.
	Completeness *float64 `json:"completeness" extensions:"x-nullable"`
}

// ComplianceRow is one church across the period's months.
type ComplianceRow struct {
	ChurchID   int64            `json:"church_id"`
	ChurchName string           `json:"church_name"`
	Cells      []ComplianceCell `json:"cells"`
	ComplianceCounts
}

// ComplianceCounts: Expected counts eligible months whose report window has
// opened; the rest split it by outcome. Outside counts reports filed for months
// the church was not required to report: kept, but not part of Expected.
type ComplianceCounts struct {
	Expected      int `json:"expected"`
	Outside       int `json:"outside"`
	Submitted     int `json:"submitted"`
	Approved      int `json:"approved"`
	Pending       int `json:"pending"`
	NeedsRevision int `json:"needs_revision"`
	OnTime        int `json:"on_time"`
	Late          int `json:"late"`
	Awaiting      int `json:"awaiting"`
	Overdue       int `json:"overdue"`
	NotFiled      int `json:"not_filed"`
	Duplicates    int `json:"duplicates"`
}

type ComplianceCell struct {
	Month int `json:"month"`
	// Expected is whether the church owed this month's report (an eligible month whose window has opened).
	Expected     bool   `json:"expected"`
	State        string `json:"state" enums:"not_required,not_reviewed,inactive,not_yet_open,awaiting_submission,overdue_grace,not_filed,submitted"`
	ReportID     int64  `json:"report_id,omitempty"`
	ReviewStatus string `json:"review_status,omitempty" enums:"approved,pending,needs_revision"`
	Timeliness   string `json:"timeliness,omitempty" enums:"on_time,late,after_cutoff,before_open"`
	SubmittedOn  string `json:"submitted_on,omitempty"`
	// Duplicates counts other reports filed for the same church and month (not counted).
	Duplicates int `json:"duplicates,omitempty"`
}

type ComplianceTotals struct {
	ComplianceCounts
	ChurchesWithSubmission int `json:"churches_with_submission"`
	ChurchesNeedingReview  int `json:"churches_needing_review"`
}

// Comparison is the same months of the previous year, under the same filters.
// LikeForLike limits the selected metric to churches with a value in both years.
type Comparison struct {
	Year        int             `json:"year"`
	Coverage    Coverage        `json:"coverage"`
	Metrics     []MetricSummary `json:"metrics"`
	Trend       []TrendPoint    `json:"trend"`
	Slots       []SlotPoint     `json:"slots"`
	LikeForLike LikeForLike     `json:"like_for_like"`
}

type LikeForLike struct {
	Churches             int     `json:"churches"`
	CurrentTotal         float64 `json:"current_total"`
	CurrentObservations  int     `json:"current_observations"`
	PreviousTotal        float64 `json:"previous_total"`
	PreviousObservations int     `json:"previous_observations"`
	// Change compares averages per church-service (Attendance) or totals; nil when not valid.
	Change *float64 `json:"change,omitempty" extensions:"x-nullable"`
}

// ChurchRecord is a church as the store reads it.
type ChurchRecord struct {
	ID         int64
	Name       string
	StartMonth *time.Time
	EndMonth   *time.Time
	Accounts   []ChurchAccountInfo
}

type ChurchAccountInfo struct {
	UserID  int64
	Name    string
	Status  string
	Deleted bool
}

// ReportData is one report with its weekly values: Metrics[metric][week-1], nil when blank.
type ReportData struct {
	ID         int64
	ChurchID   int64
	ChurchName string
	Worker     string
	Year       int
	Month      int
	Status     string
	CreatedAt  time.Time
	UpdatedAt  time.Time
	Metrics    map[string]*[5]*float64
	// Canonical is false for a second report of the same church and month (D-A31): only
	// one per church-month is counted, the approved one, else the latest updated.
	Canonical bool
}

// ChurchReportingRequest is PUT /admin/churches/:id/reporting: per-church overrides
// as "YYYY-MM" activity months. An empty start uses the organization default; an
// empty end is open-ended.
type ChurchReportingRequest struct {
	StartMonth string `json:"start_month" validate:"omitempty,datetime=2006-01"`
	EndMonth   string `json:"end_month" validate:"omitempty,datetime=2006-01"`
}

func AnalyticsPeriodBounds(period string) (int, int, bool) {
	start, ok := map[string][2]int{
		AttendancePeriodFullYear: {1, 12}, "q1": {1, 3}, "q2": {4, 6}, "q3": {7, 9}, "q4": {10, 12},
		"january": {1, 1}, "february": {2, 2}, "march": {3, 3}, "april": {4, 4}, "may": {5, 5}, "june": {6, 6},
		"july": {7, 7}, "august": {8, 8}, "september": {9, 9}, "october": {10, 10}, "november": {11, 11}, "december": {12, 12},
	}[period]
	return start[0], start[1], ok
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
	UsersActive          int64  `json:"users_active"`
	AdminsTotal          int64  `json:"admins_total"`
	ExpectedReports      int64  `json:"expected_reports"`
	NoReport             int64  `json:"no_report"`
	ChurchesTotal        int64  `json:"churches_total"`
	// Every role, by id, with its accounts; a role nobody holds counts 0.
	RoleCounts []RoleCount `json:"role_counts"`
	// Every account status, in user.UserStatuses order, zeros included.
	StatusCounts []StatusCount `json:"status_counts"`
} // @name github_com_ssr0016_pastoral-reports_backend_internal_model.AdminStats

// RoleCount is the number of accounts, deleted ones left out, holding one role.
type RoleCount struct {
	RoleID int64  `json:"role_id"`
	Name   string `json:"name"`
	Count  int64  `json:"count"`
}

// StatusCount is the number of accounts, deleted ones left out, with one status.
type StatusCount struct {
	Status string `json:"status"`
	Count  int64  `json:"count"`
}

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
	// CreatedAt is when the account was made; the Excel compliance sheet skips earlier months.
	CreatedAt time.Time `json:"-"`
} // @name github_com_ssr0016_pastoral-reports_backend_internal_model.TrackerRow

// ExportFilename names an analytics download by kind, year, period, status and
// church scope, e.g. "amgc-weekly-detail_2026_full-year_approved_3-churches.csv".
// q has been validated, so the name is plain ASCII.
func ExportFilename(q AttendanceQuery, kind, ext string) string {
	status := "approved"
	if q.Status == AttendanceStatusAll {
		status = "all-submitted"
	}
	scope := "all-churches"
	if n := len(q.ChurchIDs); n > 0 {
		scope = fmt.Sprintf("%d-churches", n)
	}
	return fmt.Sprintf("amgc-%s_%d_%s_%s_%s.%s", strings.ReplaceAll(kind, "_", "-"), q.Year, strings.ReplaceAll(q.Period, "_", "-"), status, scope, ext)
}
