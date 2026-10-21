package auditimpl

import (
	"context"
	"fmt"

	"github.com/ssr0016/pastoral-reports/backend/internal/audit"
	"github.com/ssr0016/pastoral-reports/backend/internal/database"
)

type store struct{ db *database.DB }

// NewStore returns the audit.Store over db.
func NewStore(db *database.DB) audit.Store { return &store{db: db} }

// Insert appends one audit row. Pass the transaction of the change being
// audited so both commit or roll back together (D-10). ID and CreatedAt are
// set by the database. Details must never hold narrative or prayer text.
func (r *store) Insert(ctx context.Context, e audit.Log) error {
	details := e.Details
	if details == nil {
		details = map[string]any{} // column is NOT NULL
	}
	query, args, err := r.db.Builder.
		Insert("audit_logs").
		Columns("performed_by", "performed_by_name", "performed_by_role", "action",
			"target_type", "target_id", "target_name", "details", "ip_address").
		Values(e.PerformedBy, e.PerformedByName, e.PerformedByRole, e.Action,
			e.TargetType, e.TargetID, e.TargetName, details, e.IPAddress).
		ToSql()
	if err != nil {
		return fmt.Errorf("build query: %w", err)
	}
	if _, err := r.db.Querier(ctx).Exec(ctx, query, args...); err != nil {
		return fmt.Errorf("insert audit log: %w", err)
	}
	return nil
}
