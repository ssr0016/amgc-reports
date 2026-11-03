package worker

import (
	"context"
	"testing"
	"time"

	"github.com/ssr0016/pastoral-reports/backend/internal/email"
	"github.com/ssr0016/pastoral-reports/backend/internal/report/outbox"
)

type emptyQueue struct{}

func (emptyQueue) ClaimNext(context.Context, time.Time, time.Duration) (*outbox.Job, error) {
	return nil, nil
}
func (emptyQueue) MarkSent(context.Context, int64, time.Time, time.Time) error           { return nil }
func (emptyQueue) MarkFailed(context.Context, int64, time.Time, time.Time, string) error { return nil }
func (emptyQueue) MarkSkipped(context.Context, int64, time.Time, string) error           { return nil }

type recordingScheduler struct{ calls []time.Time }

func (s *recordingScheduler) ScheduleDailyDigest(_ context.Context, day time.Time, _ string) error {
	s.calls = append(s.calls, day)
	return nil
}

type emptyMailer struct{}

func (emptyMailer) Send(context.Context, email.Message) error { return nil }

func TestWorker_SchedulesDigestEveryMinute(t *testing.T) {
	cases := []struct {
		name string
		now  time.Time
	}{
		{"before production slot", time.Date(2026, 11, 2, 23, 59, 0, 0, time.UTC)},
		{"UTC midnight", time.Date(2026, 11, 3, 0, 0, 0, 0, time.UTC)},
		{"after production slot", time.Date(2026, 11, 3, 8, 5, 0, 0, time.FixedZone("UTC", 0))},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			scheduler := &recordingScheduler{}
			w := New(emptyQueue{}, emptyMailer{}, Options{Now: func() time.Time { return tc.now }, DigestScheduler: scheduler})
			if err := w.RunOnce(context.Background()); err != nil {
				t.Fatal(err)
			}
			if len(scheduler.calls) != 1 {
				t.Fatalf("scheduled %d digests, want 1", len(scheduler.calls))
			}
		})
	}
}
