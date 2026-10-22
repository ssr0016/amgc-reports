package auditimpl

import (
	"context"
	"fmt"
	"slices"
	"time"

	"github.com/ssr0016/pastoral-reports/backend/internal/audit"
	"github.com/ssr0016/pastoral-reports/backend/internal/report"
	"github.com/ssr0016/pastoral-reports/backend/pkg/pagination"
)

// service turns a month into its Asia/Manila time range: a row written at 00:30
// on 1 October in Manila (still 30 September in UTC) belongs to October.
type service struct {
	reader audit.Reader
	loc    *time.Location
}

// NewService returns the audit.Service over reader.
func NewService(reader audit.Reader) audit.Service {
	loc, err := time.LoadLocation("Asia/Manila")
	if err != nil {
		panic(fmt.Sprintf("load Asia/Manila (is tzdata installed?): %v", err))
	}
	return &service{reader: reader, loc: loc}
}

func (s *service) List(ctx context.Context, q audit.ListQuery, pg pagination.Params) ([]audit.Log, int64, error) {
	month, ok := report.ParseMonth(q.Month)
	if !ok || q.Year < 2000 || q.Year > 2100 {
		return nil, 0, audit.ErrPeriod
	}
	if q.Action != nil && !slices.Contains(audit.Actions, *q.Action) {
		return nil, 0, audit.ErrAction
	}
	from := time.Date(q.Year, month, 1, 0, 0, 0, 0, s.loc)
	rows, total, err := s.reader.List(ctx, from, from.AddDate(0, 1, 0), q.Action, pg)
	if err != nil {
		return nil, 0, fmt.Errorf("list audit logs: %w", err)
	}
	return rows, total, nil
}
