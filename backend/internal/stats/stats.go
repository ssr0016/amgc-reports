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
	// Tracker returns who has reported for the period in q, with the same
	// period rules and ErrPeriod as Get (D-A3).
	Tracker(ctx context.Context, q GetStatsQuery) (Tracker, error)
}

// Store is the read side: one query per answer, for one month.
type Store interface {
	Counts(ctx context.Context, month string, year int) (AdminStats, error)
	// Tracker lists every church account (report.ChurchAccount), whatever its
	// status, with its report status for the month, by church then name.
	Tracker(ctx context.Context, month string, year int) ([]TrackerRow, error)
}
