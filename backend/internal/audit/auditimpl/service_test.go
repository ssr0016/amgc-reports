package auditimpl

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/ssr0016/pastoral-reports/backend/internal/audit"
	"github.com/ssr0016/pastoral-reports/backend/pkg/pagination"
)

// recordingReader remembers the range it was asked for.
type recordingReader struct {
	from, to time.Time
	action   *string
	called   bool
}

func (r *recordingReader) List(_ context.Context, from, to time.Time, action *string, _ pagination.Params) ([]audit.Log, int64, error) {
	r.from, r.to, r.action, r.called = from, to, action, true
	return []audit.Log{{ID: 1}}, 1, nil
}

func TestService_List(t *testing.T) {
	manila, err := time.LoadLocation("Asia/Manila")
	if err != nil {
		t.Fatal(err)
	}
	str := func(s string) *string { return &s }

	tests := []struct {
		name     string
		q        audit.ListQuery
		wantFrom time.Time
		wantTo   time.Time
		wantErr  error
	}{
		{"a month in Manila time", audit.ListQuery{Month: "september", Year: 2026},
			time.Date(2026, 9, 1, 0, 0, 0, 0, manila), time.Date(2026, 10, 1, 0, 0, 0, 0, manila), nil},
		{"December runs into the next year", audit.ListQuery{Month: "December", Year: 2026},
			time.Date(2026, 12, 1, 0, 0, 0, 0, manila), time.Date(2027, 1, 1, 0, 0, 0, 0, manila), nil},
		{"with an action", audit.ListQuery{Month: "May", Year: 2026, Action: str(audit.ActionLoginFailed)},
			time.Date(2026, 5, 1, 0, 0, 0, 0, manila), time.Date(2026, 6, 1, 0, 0, 0, 0, manila), nil},
		{"unknown month", audit.ListQuery{Month: "Smarch", Year: 2026}, time.Time{}, time.Time{}, audit.ErrPeriod},
		{"missing month", audit.ListQuery{Year: 2026}, time.Time{}, time.Time{}, audit.ErrPeriod},
		{"year too early", audit.ListQuery{Month: "May", Year: 1999}, time.Time{}, time.Time{}, audit.ErrPeriod},
		{"year too late", audit.ListQuery{Month: "May", Year: 2101}, time.Time{}, time.Time{}, audit.ErrPeriod},
		{"unknown action", audit.ListQuery{Month: "May", Year: 2026, Action: str("DROP_TABLE")}, time.Time{}, time.Time{}, audit.ErrAction},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := &recordingReader{}
			_, total, err := NewService(r).List(context.Background(), tt.q, pagination.Params{Page: 1, Limit: 50})
			if !errors.Is(err, tt.wantErr) {
				t.Fatalf("List() error = %v, want %v", err, tt.wantErr)
			}
			if tt.wantErr != nil {
				if r.called {
					t.Error("a rejected query must not reach the store")
				}
				return
			}
			if !r.from.Equal(tt.wantFrom) || !r.to.Equal(tt.wantTo) {
				t.Errorf("range = [%v, %v), want [%v, %v)", r.from, r.to, tt.wantFrom, tt.wantTo)
			}
			if total != 1 {
				t.Errorf("total = %d, want 1", total)
			}
			if (r.action == nil) != (tt.q.Action == nil) {
				t.Errorf("action = %v, want %v", r.action, tt.q.Action)
			}
		})
	}
}
