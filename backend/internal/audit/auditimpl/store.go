package auditimpl

import (
	"context"
	"fmt"
	"time"

	sq "github.com/Masterminds/squirrel"

	"github.com/ssr0016/pastoral-reports/backend/internal/audit"
	"github.com/ssr0016/pastoral-reports/backend/internal/database"
	"github.com/ssr0016/pastoral-reports/backend/pkg/pagination"
)

type store struct{ db *database.DB }

// NewStore returns the audit.Store over db.
func NewStore(db *database.DB) audit.Store { return &store{db: db} }

// NewReader returns the audit.Reader over db.
func NewReader(db *database.DB) audit.Reader { return &store{db: db} }

// Insert appends one audit row. Pass the transaction of the change being
// audited so both commit or roll back together (D-10). ID and CreatedAt are
// set by the database. A details key outside audit.DetailKeys is refused, so
// the change being audited rolls back instead of storing it.
func (r *store) Insert(ctx context.Context, e audit.Log) error {
	if err := e.CheckDetails(); err != nil {
		return err
	}
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

// List returns one page of rows created in [from, to), newest first, and the
// total. id breaks ties on created_at, so paging never repeats or skips a row.
// ip_address is not selected: the API never returns it (D-A4).
func (r *store) List(ctx context.Context, from, to time.Time, action *string, pg pagination.Params) ([]audit.Log, int64, error) {
	where := sq.And{sq.GtOrEq{"created_at": from}, sq.Lt{"created_at": to}}
	if action != nil {
		where = append(where, sq.Eq{"action": *action})
	}

	countQ, countArgs, err := r.db.Builder.Select("COUNT(*)").From("audit_logs").Where(where).ToSql()
	if err != nil {
		return nil, 0, fmt.Errorf("build count query: %w", err)
	}
	var total int64
	if err := r.db.Querier(ctx).QueryRow(ctx, countQ, countArgs...).Scan(&total); err != nil {
		return nil, 0, fmt.Errorf("count audit logs: %w", err)
	}

	listQ, listArgs, err := r.db.Builder.
		Select("id", "performed_by", "performed_by_name", "performed_by_role", "action",
			"target_type", "target_id", "target_name", "details", "created_at").
		From("audit_logs").
		Where(where).
		OrderBy("created_at DESC", "id DESC").
		Limit(uint64(pg.Limit)).     // #nosec G115 - limit validated
		Offset(uint64(pg.Offset())). // #nosec G115 - offset validated
		ToSql()
	if err != nil {
		return nil, 0, fmt.Errorf("build query: %w", err)
	}
	rows, err := r.db.Querier(ctx).Query(ctx, listQ, listArgs...)
	if err != nil {
		return nil, 0, fmt.Errorf("list audit logs: %w", err)
	}
	defer rows.Close()
	out := []audit.Log{}
	for rows.Next() {
		var e audit.Log
		if err := rows.Scan(&e.ID, &e.PerformedBy, &e.PerformedByName, &e.PerformedByRole, &e.Action,
			&e.TargetType, &e.TargetID, &e.TargetName, &e.Details, &e.CreatedAt); err != nil {
			return nil, 0, fmt.Errorf("scan audit log: %w", err)
		}
		out = append(out, e)
	}
	if err := rows.Err(); err != nil {
		return nil, 0, fmt.Errorf("list audit logs: %w", err)
	}
	return out, total, nil
}
