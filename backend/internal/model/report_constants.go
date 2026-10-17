package model

// Months are the English month names allowed by chk_reports_month (exact casing).
var Months = []string{
	"January", "February", "March", "April", "May", "June",
	"July", "August", "September", "October", "November", "December",
}

// MetricKeys are the 21 snake_case metric keys of chk_report_metrics_metric (D-6).
// This list is the source of truth: a test compares it with the DTO validate tag
// and another with the CHECK constraint. Adding a metric also needs a migration.
var MetricKeys = []string{
	"worship_service", "sunday_school", "prayer_meeting", "bible_studies",
	"mens_fellowship", "womens_fellowship", "youth_fellowship", "children_fellowship",
	"tithes_offering", "home_visited", "bible_study_group_led", "sermon_preached",
	"person_newly_contacted", "person_followed_up", "person_evangelized", "outreach",
	"training", "leadership", "baptism", "other", "family_day",
}

// Audit actions allowed by chk_audit_logs_action.
const (
	AuditCreateReport     = "CREATE_REPORT"
	AuditUpdateReport     = "UPDATE_REPORT"
	AuditDeleteReport     = "DELETE_REPORT"
	AuditApproveReport    = "APPROVE_REPORT"
	AuditUnapproveReport  = "UNAPPROVE_REPORT"
	AuditDownloadSingle   = "DOWNLOAD_SINGLE_EXCEL"
	AuditDownloadBulk     = "DOWNLOAD_BULK_EXCEL"
	AuditCreateUser       = "CREATE_USER"
	AuditUpdateUser       = "UPDATE_USER"
	AuditChangePassword   = "CHANGE_PASSWORD"
	AuditChangeUserStatus = "CHANGE_USER_STATUS"
	AuditTargetTypeReport = "Report"
	AuditTargetTypeUser   = "User"
)

var AuditActions = []string{
	AuditCreateReport, AuditUpdateReport, AuditDeleteReport,
	AuditApproveReport, AuditUnapproveReport,
	AuditDownloadSingle, AuditDownloadBulk,
	AuditCreateUser, AuditUpdateUser, AuditChangePassword, AuditChangeUserStatus,
}
