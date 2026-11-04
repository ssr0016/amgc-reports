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

func TestWorker_SchedulesDigestOnceAtEightManilaTime(t *testing.T) {
	manila, err := time.LoadLocation("Asia/Manila")
	if err != nil {
		t.Fatal(err)
	}
	now := time.Date(2026, time.November, 3, 7, 59, 0, 0, manila)
	scheduler := &recordingScheduler{}
	w := New(emptyQueue{}, emptyMailer{}, Options{Now: func() time.Time { return now }, DigestScheduler: scheduler})

	if err := w.RunOnce(context.Background()); err != nil {
		t.Fatal(err)
	}
	if len(scheduler.calls) != 0 {
		t.Fatalf("scheduled %d digests at 07:59, want 0", len(scheduler.calls))
	}

	now = time.Date(2026, time.November, 3, 8, 0, 0, 0, manila)
	if err := w.RunOnce(context.Background()); err != nil {
		t.Fatal(err)
	}
	if len(scheduler.calls) != 1 {
		t.Fatalf("scheduled %d digests at 08:00, want 1", len(scheduler.calls))
	}

	now = time.Date(2026, time.November, 3, 8, 1, 0, 0, manila)
	if err := w.RunOnce(context.Background()); err != nil {
		t.Fatal(err)
	}
	if len(scheduler.calls) != 1 {
		t.Fatalf("scheduled %d digests at 08:01, want 1", len(scheduler.calls))
	}
}
