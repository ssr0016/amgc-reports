// Package audit is the append-only audit trail: who did what to which record.
// Services depend on Store and write the row in the same transaction as the
// change they audit; auditimpl implements it. There is no service or HTTP
// endpoint here: nothing reads the trail through the API yet.
package audit

import "context"

// Store appends audit rows.
type Store interface {
	// Insert appends one row. Pass the ctx of the transaction of the change being
	// audited (database.DB.InTransaction), so both commit or roll back together (D-10).
	Insert(ctx context.Context, e Log) error
}
