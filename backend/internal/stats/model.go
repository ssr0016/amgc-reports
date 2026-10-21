package stats

import "errors"

// ErrPeriod is returned when only one of month and year is given, or the month
// is not an English month name. The handler maps it to 422.
var ErrPeriod = errors.New("month and year must be given together, and the month must be a month name")

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
	Month           string `json:"month"`
	Year            int    `json:"year"`
	ReportsTotal    int64  `json:"reports_total"`
	ReportsApproved int64  `json:"reports_approved"`
	ReportsPending  int64  `json:"reports_pending"`
	UsersTotal      int64  `json:"users_total"`
	AdminsTotal     int64  `json:"admins_total"`
} // @name github_com_ssr0016_pastoral-reports_backend_internal_model.AdminStats
