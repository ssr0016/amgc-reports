package audit

import (
	"errors"
	"fmt"
	"slices"
	"time"
)

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
	ActionLogin              = "LOGIN"
	ActionLogout             = "LOGOUT"
	ActionLoginFailed        = "LOGIN_FAILED"
	ActionCreateRole         = "CREATE_ROLE"
	ActionUpdateRole         = "UPDATE_ROLE"
	ActionDeleteRole         = "DELETE_ROLE"
	ActionChangeUserRole     = "CHANGE_USER_ROLE"
)

// Reasons stored in a LOGIN_FAILED row's details. A login refused by the
// account's status stores the status itself (inactive, disciplinary, deceased).
const (
	ReasonUnknownAccount = "unknown_account"
	ReasonWrongPassword  = "wrong_password"
	ReasonLocked         = "locked"
)

// Target types of the row's subject.
const (
	TargetTypeReport = "Report"
	TargetTypeUser   = "User"
	TargetTypeRole   = "Role"
)

// Actions lists every action above; a test compares it with the CHECK constraint.
var Actions = []string{
	ActionCreateReport, ActionUpdateReport, ActionDeleteReport,
	ActionApproveReport, ActionUnapproveReport,
	ActionDownloadSingle, ActionDownloadBulk,
	ActionCreateUser, ActionUpdateUser, ActionChangePassword, ActionChangeUserStatus,
	ActionAdminResetPassword, ActionDeleteUser, ActionRestoreUser,
	ActionLogin, ActionLogout, ActionLoginFailed,
	ActionCreateRole, ActionUpdateRole, ActionDeleteRole, ActionChangeUserRole,
}

// DetailKeys is the allow-list of details keys per action. Insert refuses any
// other key, so a password, token or narrative text can never reach the table.
var DetailKeys = map[string][]string{
	ActionCreateReport:       {"month", "year", "count"},
	ActionUpdateReport:       {"month", "year", "count"},
	ActionDeleteReport:       {"month", "year", "count"},
	ActionApproveReport:      {"month", "year", "count"},
	ActionUnapproveReport:    {"month", "year", "count"},
	ActionDownloadSingle:     {"month", "year", "count"},
	ActionDownloadBulk:       {"month", "year", "count"},
	ActionCreateUser:         {"role_id"},
	ActionUpdateUser:         {},
	ActionChangePassword:     {"sessions_ended"},
	ActionChangeUserStatus:   {"from", "to", "sessions_revoked"},
	ActionAdminResetPassword: {"sessions_ended"},
	ActionDeleteUser:         {"reason", "sessions_revoked"},
	ActionRestoreUser:        {"deleted_at", "deleted_by", "deleted_reason"},
	ActionLogin:              {},
	ActionLogout:             {},
	ActionLoginFailed:        {"reason"},
	ActionCreateRole:         {"name", "permissions"},
	ActionUpdateRole:         {"from", "to", "added", "removed"},
	ActionDeleteRole:         {"name"},
	ActionChangeUserRole:     {"from", "to"},
}

// ErrDetailKey is returned for an unknown action or a details key outside DetailKeys.
var ErrDetailKey = errors.New("audit details key not allowed")

// CheckDetails reports whether every details key is allowed for the row's action.
func (e Log) CheckDetails() error {
	allowed, ok := DetailKeys[e.Action]
	if !ok {
		return fmt.Errorf("%w: unknown action %q", ErrDetailKey, e.Action)
	}
	for k := range e.Details {
		if !slices.Contains(allowed, k) {
			return fmt.Errorf("%w: %q on %s", ErrDetailKey, k, e.Action)
		}
	}
	return nil
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

// LogResponse is one row as GET /admin/audit-logs returns it. It has no IP
// address (D-A4) and no actor ID: the name and role are what was stored then.
type LogResponse struct {
	ID              int64          `json:"id"`
	PerformedByName *string        `json:"performed_by_name"`
	PerformedByRole *string        `json:"performed_by_role"`
	Action          string         `json:"action"`
	TargetType      string         `json:"target_type"`
	TargetID        *string        `json:"target_id"`
	TargetName      *string        `json:"target_name"`
	Details         map[string]any `json:"details"`
	CreatedAt       time.Time      `json:"created_at"`
} // @name github_com_ssr0016_pastoral-reports_backend_internal_model.AuditLogResponse

// ToResponse maps a row to its API shape; details are never null.
func (e Log) ToResponse() LogResponse {
	details := e.Details
	if details == nil {
		details = map[string]any{}
	}
	return LogResponse{
		ID: e.ID, PerformedByName: e.PerformedByName, PerformedByRole: e.PerformedByRole,
		Action: e.Action, TargetType: e.TargetType, TargetID: e.TargetID, TargetName: e.TargetName,
		Details: details, CreatedAt: e.CreatedAt,
	}
}
