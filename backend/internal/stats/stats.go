// Package stats is the admin panel's counts: reports for one month, and the
// account totals. Callers depend on the interfaces here; statsimpl implements
// them and rest serves them.
package stats

import "context"

// Service answers GET /admin/stats.
type Service interface {
	// Get returns the counts for the period in q; an empty q means the previous
	// month in Asia/Manila. ErrPeriod when the period is malformed.
	Get(ctx context.Context, q GetStatsQuery) (AdminStats, error)
}

// Store is the read side: one query for the counts of one month.
type Store interface {
	Counts(ctx context.Context, month string, year int) (AdminStats, error)
}
