package audit

import "time"

// Actions allowed by chk_audit_logs_action. A new action needs a migration that
// widens the CHECK, shipped in the same deploy as the code that writes it.
// (The stored values are unchanged; only the Go names moved from model.Audit*.)
const (
	ActionCreateReport       = "CREATE_REPORT"
	ActionUpdateReport       = "UPDATE_REPORT"
	ActionDeleteReport       = "DELETE_REPORT"
	ActionApproveReport      = "APPROVE_REPORT"
	ActionUnapproveReport    = "UNAPPROVE_REPORT"
	ActionDownloadSingle     = "DOWNLOAD_SINGLE_EXCEL"
	ActionDownloadBulk       = "DOWNLOAD_BULK_EXCEL"
	ActionCreateUser         = "CREATE_USER"
	ActionUpdateUser         = "UPDATE_USER"
	ActionChangePassword     = "CHANGE_PASSWORD"
	ActionChangeUserStatus   = "CHANGE_USER_STATUS"
	ActionAdminResetPassword = "ADMIN_RESET_PASSWORD"
	ActionDeleteUser         = "DELETE_USER"
	ActionRestoreUser        = "RESTORE_USER"
)

// Target types of the row's subject.
const (
	TargetTypeReport = "Report"
	TargetTypeUser   = "User"
)

// Actions lists every action above; a test compares it with the CHECK constraint.
var Actions = []string{
	ActionCreateReport, ActionUpdateReport, ActionDeleteReport,
	ActionApproveReport, ActionUnapproveReport,
	ActionDownloadSingle, ActionDownloadBulk,
	ActionCreateUser, ActionUpdateUser, ActionChangePassword, ActionChangeUserStatus,
	ActionAdminResetPassword, ActionDeleteUser, ActionRestoreUser,
}

// Log is one audit row; it maps to the audit_logs table.
type Log struct {
	ID              int64          `db:"id"`
	PerformedBy     *int64         `db:"performed_by"`
	PerformedByName *string        `db:"performed_by_name"`
	PerformedByRole *string        `db:"performed_by_role"`
	Action          string         `db:"action"`
	TargetType      string         `db:"target_type"`
	TargetID        *string        `db:"target_id"`
	TargetName      *string        `db:"target_name"`
	Details         map[string]any `db:"details"`
	IPAddress       *string        `db:"ip_address"`
	CreatedAt       time.Time      `db:"created_at"`
}
