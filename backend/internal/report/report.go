// Package report is the monthly pastoral report: one per user, month and year,
// with weekly metrics. Callers depend on the interfaces here; reportimpl
// implements them and rest serves them.
package report

import (
	"context"
	"errors"

	"github.com/ssr0016/pastoral-reports/backend/pkg/pagination"
)

var (
	// ErrReportNotFound is returned for a missing, soft-deleted or someone
	// else's report. The handler maps it to 404 so existence is not revealed (D-1).
	ErrReportNotFound = errors.New("report not found")
	// ErrReportExists is returned when the owner already has an active report
	// for that month and year. The handler maps it to 409.
	ErrReportExists = errors.New("a report for this month and year already exists")
	// ErrPeriodNotAllowed is returned when a caller without reports:write_any
	// files a report for any month but the previous one. The handler maps it to 422.
	ErrPeriodNotAllowed = errors.New("reports can only be created for the previous month")
)

// Service holds the report business rules: year default, ownership scope
// (D-1), and one transaction per mutation covering report, metrics and audit
// (D-10, D-14). ip is recorded in the audit row.
type Service interface {
	Create(ctx context.Context, userID int64, ip string, req CreateReportRequest) (*ReportResponse, error)
	Get(ctx context.Context, userID, id int64) (*ReportResponse, error)
	List(ctx context.Context, userID int64, p ListReportsParams, pg pagination.Params) ([]ReportResponse, int64, error)
	Update(ctx context.Context, userID int64, ip string, id int64, req UpdateReportRequest) (*ReportResponse, error)
	Delete(ctx context.Context, userID int64, ip string, id int64) error
	ToggleComplete(ctx context.Context, userID int64, ip string, id int64) (*ReportMetadata, error)
}

// Store defines the interface for report data access. Get returns
// (nil, nil) for a missing or soft-deleted report; Update, SoftDelete and
// ToggleComplete return an error wrapping ErrNotFound instead. year is
// resolved by the service (the request may omit it).
type Store interface {
	Create(ctx context.Context, userID int64, year int, req CreateReportRequest) (*Report, error)
	Get(ctx context.Context, id int64) (*Report, error)
	List(ctx context.Context, p ListReportsParams, pg pagination.Params) ([]*Report, int64, error)
	Update(ctx context.Context, id int64, year int, req UpdateReportRequest) (*Report, error)
	SoftDelete(ctx context.Context, id int64) error
	ToggleComplete(ctx context.Context, id, approverID int64) (*Report, error)
}
