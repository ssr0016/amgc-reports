// Package audit is the append-only audit trail: who did what to which record.
// Services depend on Store and write the row in the same transaction as the
// change they audit; auditimpl implements it. Admins read the trail through
// Service (GET /admin/audit-logs); nothing can change or delete a row.
package audit

import (
	"context"
	"errors"
	"time"

	"github.com/ssr0016/pastoral-reports/backend/pkg/pagination"
)

// Store appends audit rows.
type Store interface {
	// Insert appends one row. Pass the ctx of the transaction of the change being
	// audited (database.DB.InTransaction), so both commit or roll back together (D-10).
	// A details key outside DetailKeys is refused with ErrDetailKey.
	Insert(ctx context.Context, e Log) error
}

// Reader is the read side: one page of rows created in [from, to), newest first.
// The IP address is never read.
type Reader interface {
	List(ctx context.Context, from, to time.Time, action *string, pg pagination.Params) ([]Log, int64, error)
}

// Service answers GET /admin/audit-logs.
type Service interface {
	// List returns one page of the month in q (Asia/Manila) and the total.
	// ErrPeriod for a malformed month or year, ErrAction for an unknown action.
	List(ctx context.Context, q ListQuery, pg pagination.Params) ([]Log, int64, error)
}

// ListQuery is one month, optionally narrowed to one action.
type ListQuery struct {
	Month  string // English month name, any case
	Year   int
	Action *string
}

var (
	ErrPeriod = errors.New("month must be an English month name and year between 2000 and 2100")
	ErrAction = errors.New("action is not a known audit action")
)
