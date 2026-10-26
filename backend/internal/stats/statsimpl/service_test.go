package statsimpl

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/ssr0016/pastoral-reports/backend/internal/stats"
)

// recordingStats remembers the period it was asked for and answers with fixed counts.
type recordingStats struct {
	month string
	year  int
	err   error
}

func (r *recordingStats) Counts(_ context.Context, month string, year int) (stats.AdminStats, error) {
	r.month, r.year = month, year
	return stats.AdminStats{Month: month, Year: year, ReportsTotal: 3, ReportsApproved: 1, ReportsPending: 2}, r.err
}

func (r *recordingStats) Tracker(_ context.Context, month string, year int) ([]stats.TrackerRow, error) {
	r.month, r.year = month, year
	return []stats.TrackerRow{{UserID: 1, Name: "Ana", SubmissionStatus: stats.TrackerSubmitted, ReviewStatus: stats.TrackerPending}}, r.err
}

func (r *recordingStats) AnalyticsAttendance(_ context.Context, q stats.AttendanceQuery) (stats.AttendanceAnalytics, error) {
	return stats.AttendanceAnalytics{Year: q.Year, Metric: q.Metric, Status: q.Status}, r.err
}

func TestService_AnalyticsAttendance(t *testing.T) {
	repo := &recordingStats{}
	svc := newService(repo)

	got, err := svc.AnalyticsAttendance(context.Background(), stats.AttendanceQuery{Year: 2026, Metric: "worship_service"})
	if err != nil {
		t.Fatal(err)
	}
	if got.Status != stats.AttendanceStatusApproved || got.Year != 2026 || got.Metric != "worship_service" {
		t.Fatalf("got %+v", got)
	}

	for name, q := range map[string]stats.AttendanceQuery{
		"missing year":   {Metric: "worship_service"},
		"unknown metric": {Year: 2026, Metric: "unknown"},
		"unknown status": {Year: 2026, Metric: "worship_service", Status: "pending"},
	} {
		t.Run(name, func(t *testing.T) {
			if _, err := svc.AnalyticsAttendance(context.Background(), q); !errors.Is(err, stats.ErrAnalyticsQuery) {
				t.Fatalf("error = %v, want ErrAnalyticsQuery", err)
			}
		})
	}
}

func TestService_Get(t *testing.T) {
	manila, err := time.LoadLocation("Asia/Manila")
	if err != nil {
		t.Fatal(err)
	}
	str := func(s string) *string { return &s }
	num := func(n int) *int { return &n }
	at := func(y int, m time.Month, d, h, min int, loc *time.Location) func() time.Time {
		return func() time.Time { return time.Date(y, m, d, h, min, 0, 0, loc) }
	}

	tests := []struct {
		name      string
		now       func() time.Time
		month     *string
		year      *int
		wantMonth string
		wantYear  int
		wantErr   error
	}{
		{"default is the previous month", at(2026, time.October, 16, 12, 0, manila), nil, nil, "September", 2026, nil},
		{"January defaults to December of last year", at(2026, time.January, 5, 12, 0, manila), nil, nil, "December", 2025, nil},
		{"an explicit period is used as given", at(2026, time.October, 16, 12, 0, manila), str("March"), num(2024), "March", 2024, nil},
		{"a lower-case month gets its canonical name", at(2026, time.October, 16, 12, 0, manila), str("march"), num(2024), "March", 2024, nil},
		{"month without year is rejected", at(2026, time.October, 16, 12, 0, manila), str("March"), nil, "", 0, stats.ErrPeriod},
		{"year without month is rejected", at(2026, time.October, 16, 12, 0, manila), nil, num(2024), "", 0, stats.ErrPeriod},
		{"an unknown month is rejected", at(2026, time.October, 16, 12, 0, manila), str("Smarch"), num(2024), "", 0, stats.ErrPeriod},
		// 2026-10-31 16:30 UTC is 2026-11-01 00:30 in Manila: November has begun there, so the previous month is October.
		{"just after midnight in Manila the month has turned", at(2026, time.October, 31, 16, 30, time.UTC), nil, nil, "October", 2026, nil},
		// 2026-10-31 15:30 UTC is 2026-10-31 23:30 in Manila: still October, so the previous month is September.
		{"just before midnight in Manila it has not", at(2026, time.October, 31, 15, 30, time.UTC), nil, nil, "September", 2026, nil},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repo := &recordingStats{}
			svc := newService(repo)
			svc.now = tt.now

			got, err := svc.Get(context.Background(), stats.GetStatsQuery{Month: tt.month, Year: tt.year})
			if !errors.Is(err, tt.wantErr) {
				t.Fatalf("error = %v, want %v", err, tt.wantErr)
			}
			if tt.wantErr != nil {
				if repo.month != "" {
					t.Errorf("the repository was queried (%q) for an invalid period", repo.month)
				}
				return
			}
			if repo.month != tt.wantMonth || repo.year != tt.wantYear {
				t.Errorf("queried %s %d, want %s %d", repo.month, repo.year, tt.wantMonth, tt.wantYear)
			}
			if got.Month != tt.wantMonth || got.Year != tt.wantYear || got.ReportsTotal != 3 {
				t.Errorf("got %+v", got)
			}
		})
	}

	t.Run("a repository error is returned wrapped", func(t *testing.T) {
		boom := errors.New("boom")
		svc := newService(&recordingStats{err: boom})
		if _, err := svc.Get(context.Background(), stats.GetStatsQuery{}); !errors.Is(err, boom) {
			t.Fatalf("error = %v, want it to wrap boom", err)
		}
	})
}

// TestService_Tracker checks Tracker shares Get's period rule; the rule itself is covered above.
func TestService_Tracker(t *testing.T) {
	manila, err := time.LoadLocation("Asia/Manila")
	if err != nil {
		t.Fatal(err)
	}
	str := func(s string) *string { return &s }
	num := func(n int) *int { return &n }
	newSvc := func(repo *recordingStats) *service {
		svc := newService(repo)
		svc.now = func() time.Time { return time.Date(2026, time.January, 5, 12, 0, 0, 0, manila) }
		return svc
	}

	t.Run("no period is the previous Manila month", func(t *testing.T) {
		repo := &recordingStats{}
		got, err := newSvc(repo).Tracker(context.Background(), stats.GetStatsQuery{})
		if err != nil {
			t.Fatal(err)
		}
		if repo.month != "December" || repo.year != 2025 || got.Month != "December" || got.Year != 2025 {
			t.Errorf("queried %s %d, got %s %d; want December 2025", repo.month, repo.year, got.Month, got.Year)
		}
		if len(got.Rows) != 1 || got.Rows[0].SubmissionStatus != stats.TrackerSubmitted || got.Rows[0].ReviewStatus != stats.TrackerPending {
			t.Errorf("rows = %+v", got.Rows)
		}
	})

	t.Run("a lower-case month gets its canonical name", func(t *testing.T) {
		repo := &recordingStats{}
		got, err := newSvc(repo).Tracker(context.Background(), stats.GetStatsQuery{Month: str("march"), Year: num(2024)})
		if err != nil {
			t.Fatal(err)
		}
		if repo.month != "March" || got.Month != "March" || got.Year != 2024 {
			t.Errorf("queried %s, got %s %d", repo.month, got.Month, got.Year)
		}
	})

	for name, q := range map[string]stats.GetStatsQuery{
		"month without year": {Month: str("March")},
		"year without month": {Year: num(2024)},
		"an unknown month":   {Month: str("Smarch"), Year: num(2024)},
	} {
		t.Run(name+" is ErrPeriod", func(t *testing.T) {
			repo := &recordingStats{}
			if _, err := newSvc(repo).Tracker(context.Background(), q); !errors.Is(err, stats.ErrPeriod) {
				t.Fatalf("error = %v, want ErrPeriod", err)
			}
			if repo.month != "" {
				t.Errorf("the repository was queried (%q) for an invalid period", repo.month)
			}
		})
	}

	t.Run("a repository error is returned wrapped", func(t *testing.T) {
		boom := errors.New("boom")
		if _, err := newSvc(&recordingStats{err: boom}).Tracker(context.Background(), stats.GetStatsQuery{}); !errors.Is(err, boom) {
			t.Fatalf("error = %v, want it to wrap boom", err)
		}
	})
}
