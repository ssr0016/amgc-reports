package repository

import (
	"context"
	"fmt"

	"github.com/ssr0016/pastoral-reports/backend/internal/database"
	"github.com/ssr0016/pastoral-reports/backend/internal/model"
)

type AuditRepo struct{ db *database.DB }

func NewAuditRepo(db *database.DB) *AuditRepo { return &AuditRepo{db: db} }

// Insert appends one audit row. Pass the transaction of the change being
// audited so both commit or roll back together (D-10). ID and CreatedAt are
// set by the database. Details must never hold narrative or prayer text.
func (r *AuditRepo) Insert(ctx context.Context, e model.AuditLog) error {
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
