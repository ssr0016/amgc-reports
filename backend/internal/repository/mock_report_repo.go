package repository

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/jackc/pgx/v5/pgtype"

	"github.com/ssr0016/pastoral-reports/backend/internal/model"
	"github.com/ssr0016/pastoral-reports/backend/pkg/pagination"
)

// MockReportRepo is an in-memory implementation of ReportRepository for testing.
// The Querier argument is ignored, so there is no transaction: a failed audit
// write in a service test does not roll the report back.
type MockReportRepo struct {
	mu      sync.Mutex
	reports map[int64]*model.Report
	nextID  int64

	// Override functions for testing specific behaviors
	CreateFunc         func(ctx context.Context, q Querier, userID int64, year int, req model.CreateReportRequest) (*model.Report, error)
	GetFunc            func(ctx context.Context, q Querier, id int64) (*model.Report, error)
	ListFunc           func(ctx context.Context, p model.ListReportsParams, pg pagination.Params) ([]*model.Report, int64, error)
	UpdateFunc         func(ctx context.Context, q Querier, id int64, year int, req model.UpdateReportRequest) (*model.Report, error)
	SoftDeleteFunc     func(ctx context.Context, q Querier, id int64) error
	ToggleCompleteFunc func(ctx context.Context, q Querier, id, approverID int64) (*model.Report, error)
}

// NewMockReportRepo creates a new mock repository.
func NewMockReportRepo() *MockReportRepo {
	return &MockReportRepo{reports: make(map[int64]*model.Report), nextID: 1}
}

// mockMetrics converts request metrics to the stored form. Unlike Postgres it does not pad to NUMERIC(12,2) scale, so 0.5 stays "0.5".
func mockMetrics(in map[string]model.MetricWeeks) map[string]map[int]pgtype.Numeric {
	out := map[string]map[int]pgtype.Numeric{}
	for metric, weeks := range in {
		for weekNo, v := range weeks.NonNil() {
			var n pgtype.Numeric
			if err := n.Scan(string(v)); err != nil {
				continue // the validator rejects bad values before the repo
			}
			if out[metric] == nil {
				out[metric] = map[int]pgtype.Numeric{}
			}
			out[metric][weekNo] = n
		}
	}
	return out
}

func (m *MockReportRepo) active(id int64) *model.Report {
	if r, ok := m.reports[id]; ok && r.DeletedAt == nil {
		return r
	}
	return nil
}

// duplicate reports whether another active report has the same owner, month and year.
func (m *MockReportRepo) duplicate(skipID, userID int64, month string, year int) bool {
	for _, r := range m.reports {
		if r.DeletedAt == nil && r.ID != skipID && r.CreatedBy == userID && r.Month == month && int(r.Year) == year {
			return true
		}
	}
	return false
}

func ptr(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}

func copyReport(r *model.Report) *model.Report {
	c := *r
	return &c
}

func (m *MockReportRepo) Create(ctx context.Context, q Querier, userID int64, year int, req model.CreateReportRequest) (*model.Report, error) {
	if m.CreateFunc != nil {
		return m.CreateFunc(ctx, q, userID, year, req)
	}
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.duplicate(0, userID, req.Month, year) {
		return nil, fmt.Errorf("create report: %w", ErrUniqueViolation)
	}
	now := time.Now()
	r := &model.Report{
		ID: m.nextID, CreatedBy: userID, Month: req.Month, Year: int16(year),
		Worker: ptr(req.Worker), AreaAssignment: ptr(req.AreaAssignment), ChurchName: ptr(req.ChurchName),
		Names: ptr(req.Names), NarrativeReport: ptr(req.NarrativeReport),
		Challenges: ptr(req.Challenges), PrayerRequest: ptr(req.PrayerRequest),
		Metrics: mockMetrics(req.Metrics), CreatedAt: now, UpdatedAt: now,
	}
	m.reports[r.ID] = r
	m.nextID++
	return copyReport(r), nil
}

func (m *MockReportRepo) Get(ctx context.Context, q Querier, id int64) (*model.Report, error) {
	if m.GetFunc != nil {
		return m.GetFunc(ctx, q, id)
	}
	m.mu.Lock()
	defer m.mu.Unlock()

	if r := m.active(id); r != nil {
		return copyReport(r), nil
	}
	return nil, nil
}

func (m *MockReportRepo) List(ctx context.Context, p model.ListReportsParams, pg pagination.Params) ([]*model.Report, int64, error) {
	if m.ListFunc != nil {
		return m.ListFunc(ctx, p, pg)
	}
	m.mu.Lock()
	defer m.mu.Unlock()

	contains := func(field *string, sub string) bool {
		return sub == "" || (field != nil && strings.Contains(strings.ToLower(*field), strings.ToLower(sub)))
	}
	var all []*model.Report
	for _, r := range m.reports {
		if r.DeletedAt != nil ||
			(p.CreatedBy != 0 && r.CreatedBy != p.CreatedBy) ||
			(p.Year != 0 && int(r.Year) != p.Year) ||
			(p.Month != "" && !strings.EqualFold(r.Month, p.Month)) ||
			!contains(r.Worker, p.Worker) || !contains(r.AreaAssignment, p.Area) || !contains(r.ChurchName, p.Church) {
			continue
		}
		all = append(all, r)
	}
	sort.Slice(all, func(i, j int) bool {
		if all[i].Year != all[j].Year {
			return all[i].Year > all[j].Year
		}
		return all[i].ID > all[j].ID
	})

	total := int64(len(all))
	start := min(pg.Offset(), len(all))
	end := min(start+pg.Limit, len(all))
	page := make([]*model.Report, 0, end-start)
	for _, r := range all[start:end] {
		page = append(page, copyReport(r))
	}
	return page, total, nil
}

func (m *MockReportRepo) Update(ctx context.Context, q Querier, id int64, year int, req model.UpdateReportRequest) (*model.Report, error) {
	if m.UpdateFunc != nil {
		return m.UpdateFunc(ctx, q, id, year, req)
	}
	m.mu.Lock()
	defer m.mu.Unlock()

	r := m.active(id)
	if r == nil {
		return nil, fmt.Errorf("report: %w", ErrNotFound)
	}
	if m.duplicate(id, r.CreatedBy, req.Month, year) {
		return nil, fmt.Errorf("update report: %w", ErrUniqueViolation)
	}
	r.Month, r.Year = req.Month, int16(year)
	r.Worker, r.AreaAssignment, r.ChurchName = ptr(req.Worker), ptr(req.AreaAssignment), ptr(req.ChurchName)
	r.Names, r.NarrativeReport = ptr(req.Names), ptr(req.NarrativeReport)
	r.Challenges, r.PrayerRequest = ptr(req.Challenges), ptr(req.PrayerRequest)
	r.Metrics = mockMetrics(req.Metrics)
	r.UpdatedAt = time.Now()
	return copyReport(r), nil
}

func (m *MockReportRepo) SoftDelete(ctx context.Context, q Querier, id int64) error {
	if m.SoftDeleteFunc != nil {
		return m.SoftDeleteFunc(ctx, q, id)
	}
	m.mu.Lock()
	defer m.mu.Unlock()

	r := m.active(id)
	if r == nil {
		return fmt.Errorf("report: %w", ErrNotFound)
	}
	now := time.Now()
	r.DeletedAt, r.UpdatedAt = &now, now
	return nil
}

func (m *MockReportRepo) ToggleComplete(ctx context.Context, q Querier, id, approverID int64) (*model.Report, error) {
	if m.ToggleCompleteFunc != nil {
		return m.ToggleCompleteFunc(ctx, q, id, approverID)
	}
	m.mu.Lock()
	defer m.mu.Unlock()

	r := m.active(id)
	if r == nil {
		return nil, fmt.Errorf("report: %w", ErrNotFound)
	}
	now := time.Now()
	if r.Completed {
		r.Completed, r.CompletedAt, r.CompletedBy = false, nil, nil
	} else {
		r.Completed, r.CompletedAt, r.CompletedBy = true, &now, &approverID
	}
	r.UpdatedAt = now
	c := copyReport(r)
	c.Metrics = nil // the real ToggleComplete does not load metrics
	return c, nil
}
