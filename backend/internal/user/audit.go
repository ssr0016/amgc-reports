package user

import (
	"strconv"

	"github.com/ssr0016/pastoral-reports/backend/internal/audit"
)

// AuditLog is the audit row for a change to target made by actor. It never carries the password or its hash: the target is named by its
// email, and details hold only non-secret facts such as the role.
func AuditLog(actor *User, ip, action string, target *User, details map[string]any) audit.Log {
	id := strconv.FormatInt(target.ID, 10)
	e := audit.Log{
		PerformedBy: &actor.ID, PerformedByName: &actor.Name,
		Action: action, TargetType: audit.TargetTypeUser, TargetID: &id, TargetName: &target.Email,
		Details: details,
	}
	if actor.Role != nil {
		e.PerformedByRole = &actor.Role.Name
	}
	if ip != "" {
		e.IPAddress = &ip
	}
	return e
}
