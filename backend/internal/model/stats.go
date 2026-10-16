package model

// AdminStats is the body of GET /admin/stats: counts for one month, plus the
// account totals, which do not depend on the month. Counts only, no report text.
type AdminStats struct {
	Month           string `json:"month"`
	Year            int    `json:"year"`
	ReportsTotal    int64  `json:"reports_total"`
	ReportsApproved int64  `json:"reports_approved"`
	ReportsPending  int64  `json:"reports_pending"`
	UsersTotal      int64  `json:"users_total"`
	AdminsTotal     int64  `json:"admins_total"`
}
