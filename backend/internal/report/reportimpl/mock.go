package reportimpl

import (
	"context"
	"fmt"
	"math"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/jackc/pgx/v5/pgtype"
	"github.com/ssr0016/pastoral-reports/backend/internal/database"
	"github.com/ssr0016/pastoral-reports/backend/internal/report"
	"github.com/ssr0016/pastoral-reports/backend/pkg/pagination"
)

// MockStore is an in-memory report.Store for testing.
// It has no transaction: a failed audit
// write in a service test does not roll the report back.
type MockStore struct {
	mu      sync.Mutex
	reports map[int64]*report.Report
	nextID  int64

	// Override functions for testing specific behaviors
	CreateFunc          func(ctx context.Context, userID int64, year int, req report.CreateReportRequest) (*report.Report, error)
	GetFunc             func(ctx context.Context, id int64) (*report.Report, error)
	ListFunc            func(ctx context.Context, p report.ListReportsParams, pg pagination.Params) ([]*report.Report, int64, error)
	UpdateFunc          func(ctx context.Context, id int64, year int, req report.UpdateReportRequest) (*report.Report, error)
	SoftDeleteFunc      func(ctx context.Context, id int64) error
	ToggleCompleteFunc  func(ctx context.Context, id, approverID int64, adminNotes ...string) (*report.Report, error)
	RequestRevisionFunc func(ctx context.Context, id, approverID int64, adminNotes ...string) (*report.Report, error)

	// Accounts are the church accounts Dashboard lists, in order; their status
	// comes from the account's report of the month in the mock.
	Accounts []MockAccount
}

// MockAccount is a church account for the mock Dashboard.
type MockAccount struct {
	ID     int64
	Church string
}

// NewMockStore creates a new mock repository.
func NewMockStore() *MockStore {
	return &MockStore{reports: make(map[int64]*report.Report), nextID: 1}
}

// mockMetrics converts request metrics to the stored form. Unlike Postgres it does not pad to NUMERIC(12,2) scale, so 0.5 stays "0.5".
func mockMetrics(in map[string]report.MetricWeeks) map[string]map[int]pgtype.Numeric {
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

func (m *MockStore) active(id int64) *report.Report {
	if r, ok := m.reports[id]; ok && r.DeletedAt == nil {
		return r
	}
	return nil
}

// duplicate reports whether another active report has the same owner, month and year.
func (m *MockStore) duplicate(skipID, userID int64, month string, year int) bool {
	for _, r := range m.reports {
		if r.DeletedAt == nil && r.ID != skipID && r.CreatedBy == userID && r.Month == month && int(r.Year) == year {
			return true
		}
	}
	return false
}

// toYear converts a year to the column type. The real column is SMALLINT with a
// CHECK of 2000 to 2100; the mock only guards the conversion, since the service
// has already validated the range.
func toYear(year int) (int16, error) {
	if year < math.MinInt16 || year > math.MaxInt16 {
		return 0, fmt.Errorf("year out of range: %d", year)
	}
	return int16(year), nil
}

func ptr(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}

func copyReport(r *report.Report) *report.Report {
	c := *r
	return &c
}

func (m *MockStore) Create(ctx context.Context, userID int64, year int, req report.CreateReportRequest) (*report.Report, error) {
	if m.CreateFunc != nil {
		return m.CreateFunc(ctx, userID, year, req)
	}
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.duplicate(0, userID, req.Month, year) {
		return nil, fmt.Errorf("create report: %w", database.ErrUniqueViolation)
	}
	y, err := toYear(year)
	if err != nil {
		return nil, fmt.Errorf("create report: %w", err)
	}
	now := time.Now()
	r := &report.Report{
		ID: m.nextID, CreatedBy: userID, Month: req.Month, Year: y,
		Worker: ptr(req.Worker), AreaAssignment: ptr(req.AreaAssignment), ChurchName: ptr(req.ChurchName),
		Names: ptr(req.Names), NarrativeReport: ptr(req.NarrativeReport),
		Challenges: ptr(req.Challenges), PrayerRequest: ptr(req.PrayerRequest),
		Metrics: mockMetrics(req.Metrics), CreatedAt: now, UpdatedAt: now,
	}
	r.Status = "pending"
	m.reports[r.ID] = r
	m.nextID++
	return copyReport(r), nil
}

func (m *MockStore) Get(ctx context.Context, id int64) (*report.Report, error) {
	if m.GetFunc != nil {
		return m.GetFunc(ctx, id)
	}
	m.mu.Lock()
	defer m.mu.Unlock()

	if r := m.active(id); r != nil {
		return copyReport(r), nil
	}
	return nil, nil
}

func (m *MockStore) List(ctx context.Context, p report.ListReportsParams, pg pagination.Params) ([]*report.Report, int64, error) {
	if m.ListFunc != nil {
		return m.ListFunc(ctx, p, pg)
	}
	m.mu.Lock()
	defer m.mu.Unlock()

	var all []*report.Report
	for _, r := range m.reports {
		if matchesReport(r, p) {
			all = append(all, r)
		}
	}
	// newest year first, then newest ID, like the real ORDER BY
	sort.Slice(all, func(i, j int) bool {
		if all[i].Year != all[j].Year {
			return all[i].Year > all[j].Year
		}
		return all[i].ID > all[j].ID
	})
	return pageOf(all, pg), int64(len(all)), nil
}

func (m *MockStore) Dashboard(_ context.Context, month string, year int) ([]report.DashboardRow, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	out := []report.DashboardRow{}
	for _, acc := range m.Accounts {
		row := report.DashboardRow{Church: acc.Church, Status: report.StatusNoReport}
		for _, r := range m.reports {
			if r.DeletedAt == nil && r.CreatedBy == acc.ID && int(r.Year) == year && strings.EqualFold(r.Month, month) {
				row.Month, row.Year, row.Status = r.Month, int(r.Year), r.Status
				switch row.Status {
				case "approved":
					row.Status = report.StatusApproved
				case "needs_revision":
					row.Status = "Needs Revision"
				case "":
					row.Status = report.StatusPending
					if r.Completed {
						row.Status = report.StatusApproved
					}
				default:
					row.Status = report.StatusPending
				}
			}
		}
		out = append(out, row)
	}
	return out, nil
}

// matchesReport applies the list filters: not deleted, owner, year, month
// (case-insensitive) and partial case-insensitive worker, area and church.
func matchesReport(r *report.Report, p report.ListReportsParams) bool {
	return r.DeletedAt == nil && matchesReportFilters(r, p) &&
		containsFold(r.Worker, p.Worker) && containsFold(r.AreaAssignment, p.Area) && containsFold(r.ChurchName, p.Church)
}

func matchesReportFilters(r *report.Report, p report.ListReportsParams) bool {
	if (p.CreatedBy != 0 && r.CreatedBy != p.CreatedBy) ||
		(p.Year != 0 && int(r.Year) != p.Year) ||
		(p.Month != "" && !strings.EqualFold(r.Month, p.Month)) ||
		(p.Completed != nil && r.Completed != *p.Completed) ||
		(p.Status != "" && r.Status != p.Status) ||
		(p.Revised && r.LastRevisedAt == nil) {
		return false
	}
	return true
}

func (m *MockStore) RequestRevision(ctx context.Context, id, approverID int64, adminNotes ...string) (*report.Report, error) {
	if m.RequestRevisionFunc != nil {
		return m.RequestRevisionFunc(ctx, id, approverID, adminNotes...)
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	r := m.active(id)
	if r == nil {
		return nil, fmt.Errorf("report: %w", database.ErrNotFound)
	}
	r.Completed, r.Status, r.CompletedAt, r.CompletedBy, r.AdminNotes = false, "needs_revision", nil, nil, notePtr(adminNotes)
	return copyReport(r), nil
}

// containsFold reports whether field contains sub, ignoring case. An empty sub matches anything.
func containsFold(field *string, sub string) bool {
	return sub == "" || (field != nil && strings.Contains(strings.ToLower(*field), strings.ToLower(sub)))
}

// pageOf returns copies of the reports on the requested page.
func pageOf(all []*report.Report, pg pagination.Params) []*report.Report {
	start := min(pg.Offset(), len(all))
	end := min(start+pg.Limit, len(all))
	page := make([]*report.Report, 0, end-start)
	for _, r := range all[start:end] {
		page = append(page, copyReport(r))
	}
	return page
}

func (m *MockStore) Update(ctx context.Context, id int64, year int, req report.UpdateReportRequest) (*report.Report, error) {
	if m.UpdateFunc != nil {
		return m.UpdateFunc(ctx, id, year, req)
	}
	m.mu.Lock()
	defer m.mu.Unlock()

	r := m.active(id)
	if r == nil {
		return nil, fmt.Errorf("report: %w", database.ErrNotFound)
	}
	if m.duplicate(id, r.CreatedBy, req.Month, year) {
		return nil, fmt.Errorf("update report: %w", database.ErrUniqueViolation)
	}
	y, err := toYear(year)
	if err != nil {
		return nil, fmt.Errorf("update report: %w", err)
	}
	r.Month, r.Year = req.Month, y
	r.Names, r.NarrativeReport = ptr(req.Names), ptr(req.NarrativeReport)
	r.Challenges, r.PrayerRequest = ptr(req.Challenges), ptr(req.PrayerRequest)
	r.Metrics = mockMetrics(req.Metrics)
	r.UpdatedAt = time.Now()
	return copyReport(r), nil
}

func (m *MockStore) SoftDelete(ctx context.Context, id int64) error {
	if m.SoftDeleteFunc != nil {
		return m.SoftDeleteFunc(ctx, id)
	}
	m.mu.Lock()
	defer m.mu.Unlock()

	r := m.active(id)
	if r == nil {
		return fmt.Errorf("report: %w", database.ErrNotFound)
	}
	now := time.Now()
	r.DeletedAt, r.UpdatedAt = &now, now
	return nil
}

func (m *MockStore) ToggleComplete(ctx context.Context, id, approverID int64, adminNotes ...string) (*report.Report, error) {
	if m.ToggleCompleteFunc != nil {
		return m.ToggleCompleteFunc(ctx, id, approverID, adminNotes...)
	}
	m.mu.Lock()
	defer m.mu.Unlock()

	r := m.active(id)
	if r == nil {
		return nil, fmt.Errorf("report: %w", database.ErrNotFound)
	}
	now := time.Now()
	if r.Completed {
		r.Completed, r.Status, r.CompletedAt, r.CompletedBy, r.AdminNotes = false, "needs_revision", nil, nil, notePtr(adminNotes)
	} else {
		r.Completed, r.Status, r.CompletedAt, r.CompletedBy, r.AdminNotes = true, "approved", &now, &approverID, nil
	}
	r.UpdatedAt = now
	c := copyReport(r)
	c.Metrics = nil // the real ToggleComplete does not load metrics
	return c, nil
}

func notePtr(notes []string) *string {
	note := firstNote(notes)
	if note == "" {
		return nil
	}
	return &note
}
