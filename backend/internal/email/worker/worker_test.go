package worker

import (
	"context"
	"testing"
	"time"

	"github.com/ssr0016/pastoral-reports/backend/internal/email"
	"github.com/ssr0016/pastoral-reports/backend/internal/email/resend"
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

type ackQueue struct {
	emptyQueue
	ctx   context.Context
	state string
}

func (q *ackQueue) MarkSent(ctx context.Context, _ int64, _, _ time.Time) error {
	q.ctx, q.state = ctx, "sent"
	return ctx.Err()
}

func (q *ackQueue) MarkFailed(ctx context.Context, _ int64, _, _ time.Time, _ string) error {
	q.ctx, q.state = ctx, "failed"
	return ctx.Err()
}

func (q *ackQueue) MarkSkipped(ctx context.Context, _ int64, _ time.Time, _ string) error {
	q.ctx, q.state = ctx, "skipped"
	return ctx.Err()
}

type contextMailer func(context.Context, email.Message) error

func (f contextMailer) Send(ctx context.Context, message email.Message) error {
	return f(ctx, message)
}

func TestWorker_AcknowledgmentsHaveIndependentDeadlines(t *testing.T) {
	for _, tt := range []struct {
		name           string
		event          string
		sendErr        error
		attempts       int
		waitForTimeout bool
		wantState      string
	}{
		{name: "sent", event: "approved", wantState: "sent"},
		{name: "failed", event: "approved", sendErr: context.DeadlineExceeded, wantState: "failed"},
		{name: "permanent error", event: "approved", sendErr: &resend.HTTPError{StatusCode: 400}, wantState: "skipped"},
		{name: "attempts exhausted", event: "approved", sendErr: context.DeadlineExceeded, attempts: 5, wantState: "skipped"},
		{name: "render error", event: "unsupported", wantState: "skipped"},
		{name: "send timeout", event: "approved", waitForTimeout: true, wantState: "failed"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			q := &ackQueue{}
			mailer := contextMailer(func(ctx context.Context, _ email.Message) error {
				if tt.waitForTimeout {
					<-ctx.Done()
					return ctx.Err()
				}
				return tt.sendErr
			})
			w := New(q, mailer, Options{InFlightTimeout: time.Millisecond})
			ctx, cancel := context.WithCancel(context.Background())
			cancel()
			if err := w.process(ctx, &outbox.Job{EventType: tt.event, Attempts: tt.attempts}); err != nil {
				t.Fatal(err)
			}
			if q.state != tt.wantState {
				t.Fatalf("acknowledged %q, want %q", q.state, tt.wantState)
			}
			deadline, ok := q.ctx.Deadline()
			if !ok {
				t.Fatal("database acknowledgment has no deadline")
			}
			if remaining := time.Until(deadline); remaining <= 0 || remaining > 10*time.Second {
				t.Fatalf("acknowledgment deadline in %v, want at most 10s", remaining)
			}
			if q.ctx.Err() != context.Canceled {
				t.Fatalf("acknowledgment context not released: %v", q.ctx.Err())
			}
		})
	}
}

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
