package service

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/ssr0016/pastoral-reports/backend/internal/model"
)

// recordingStats remembers the period it was asked for and answers with fixed counts.
type recordingStats struct {
	month string
	year  int
	err   error
}

func (r *recordingStats) Counts(_ context.Context, month string, year int) (model.AdminStats, error) {
	r.month, r.year = month, year
	return model.AdminStats{Month: month, Year: year, ReportsTotal: 3, ReportsApproved: 1, ReportsPending: 2}, r.err
}

func TestStatsService_Get(t *testing.T) {
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
		{"month without year is rejected", at(2026, time.October, 16, 12, 0, manila), str("March"), nil, "", 0, ErrStatsPeriod},
		{"year without month is rejected", at(2026, time.October, 16, 12, 0, manila), nil, num(2024), "", 0, ErrStatsPeriod},
		{"an unknown month is rejected", at(2026, time.October, 16, 12, 0, manila), str("Smarch"), num(2024), "", 0, ErrStatsPeriod},
		// 2026-10-31 16:30 UTC is 2026-11-01 00:30 in Manila: November has begun there, so the previous month is October.
		{"just after midnight in Manila the month has turned", at(2026, time.October, 31, 16, 30, time.UTC), nil, nil, "October", 2026, nil},
		// 2026-10-31 15:30 UTC is 2026-10-31 23:30 in Manila: still October, so the previous month is September.
		{"just before midnight in Manila it has not", at(2026, time.October, 31, 15, 30, time.UTC), nil, nil, "September", 2026, nil},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repo := &recordingStats{}
			svc := NewStatsService(repo)
			svc.now = tt.now

			got, err := svc.Get(context.Background(), tt.month, tt.year)
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
		svc := NewStatsService(&recordingStats{err: boom})
		if _, err := svc.Get(context.Background(), nil, nil); !errors.Is(err, boom) {
			t.Fatalf("error = %v, want it to wrap boom", err)
		}
	})
}
