//go:build integration

package worker

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ssr0016/pastoral-reports/backend/internal/database"
	"github.com/ssr0016/pastoral-reports/backend/internal/email"
	"github.com/ssr0016/pastoral-reports/backend/internal/email/resend"
	"github.com/ssr0016/pastoral-reports/backend/internal/report"
	"github.com/ssr0016/pastoral-reports/backend/internal/report/outbox"
	"github.com/ssr0016/pastoral-reports/backend/internal/report/reportimpl"
	"github.com/ssr0016/pastoral-reports/backend/internal/testutil"
	"github.com/ssr0016/pastoral-reports/backend/internal/user/userimpl"
)

type fakeMailer struct {
	messages []email.Message
	errors   []error
}

func (f *fakeMailer) Send(_ context.Context, message email.Message) error {
	f.messages = append(f.messages, message)
	if len(f.errors) == 0 {
		return nil
	}
	err := f.errors[0]
	f.errors = f.errors[1:]
	return err
}

type workerFixture struct {
	db   *database.DB
	job  report.Notification
	mail *fakeMailer
	q    outbox.Queue
}

func newWorkerFixture(t *testing.T, event string) workerFixture {
	t.Helper()
	tdb := testutil.SetupPostgres(t)
	tdb.MigrateWithGoose(t)
	db := &database.DB{Pool: tdb.Pool, Builder: database.NewStatementBuilder()}
	ctx := context.Background()
	users := userimpl.NewStore(db)
	reports := reportimpl.NewStore(db)
	owner, err := users.CreateWithPassword(ctx, "worker-owner@example.com", "Owner", "hash")
	require.NoError(t, err)
	rep, err := reports.Create(ctx, owner.ID, 2026, report.CreateReportRequest{Month: "April"})
	require.NoError(t, err)
	job := report.Notification{EventType: event, IdempotencyKey: "worker:1:1:123", ReportID: rep.ID, RecipientUserID: owner.ID, RecipientEmail: owner.Email, Month: "April", Year: 2026, AdminRemark: "Please clarify"}
	require.NoError(t, outbox.NewStore(db).Enqueue(ctx, job))
	return workerFixture{db: db, job: job, mail: &fakeMailer{}, q: outbox.NewQueue(db)}
}

func workerFor(f workerFixture, options Options) *Worker {
	if options.Now == nil {
		options.Now = time.Now
	}
	return New(f.q, f.mail, options)
}

func stateFor(t *testing.T, db *database.DB, idempotencyKey string) string {
	t.Helper()
	var state string
	require.NoError(t, db.Pool.QueryRow(context.Background(), `SELECT state FROM report_notification_outbox WHERE idempotency_key = $1`, idempotencyKey).Scan(&state))
	return state
}

func TestWorker_RunOnce_SendsPendingRow(t *testing.T) {
	f := newWorkerFixture(t, "approved")
	require.NoError(t, workerFor(f, Options{}).RunOnce(context.Background()))
	assert.Len(t, f.mail.messages, 1)
	assert.Equal(t, "worker-owner@example.com", f.mail.messages[0].To)
	assert.Equal(t, "sent", stateFor(t, f.db, f.job.IdempotencyKey))
}

func TestWorker_Run_PollsAfterTicker(t *testing.T) {
	f := newWorkerFixture(t, "approved")
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() { defer close(done); _ = workerFor(f, Options{PollInterval: time.Millisecond}).Run(ctx) }()
	require.Eventually(t, func() bool { return len(f.mail.messages) == 1 }, time.Second, time.Millisecond)
	cancel()
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("worker did not stop")
	}
}

func TestWorker_SuccessMarksSent(t *testing.T) {
	f := newWorkerFixture(t, "needs_revision")
	require.NoError(t, workerFor(f, Options{}).RunOnce(context.Background()))
	assert.Equal(t, "sent", stateFor(t, f.db, f.job.IdempotencyKey))
}

func TestWorker_RetryAfterControlsNextAttempt(t *testing.T) {
	f := newWorkerFixture(t, "approved")
	f.mail.errors = []error{&resend.HTTPError{StatusCode: 429, RetryAfter: durationPtr(17 * time.Second)}}
	now := time.Now().UTC()
	require.NoError(t, workerFor(f, Options{Now: func() time.Time { return now }}).RunOnce(context.Background()))
	var state string
	var next time.Time
	require.NoError(t, f.db.Pool.QueryRow(context.Background(), `SELECT state, next_attempt_at FROM report_notification_outbox WHERE idempotency_key = $1`, f.job.IdempotencyKey).Scan(&state, &next))
	assert.Equal(t, "failed", state)
	assert.WithinDuration(t, now.Add(17*time.Second), next, time.Second)
}

func TestWorker_ZeroRetryAfterRetriesImmediately(t *testing.T) {
	f := newWorkerFixture(t, "approved")
	f.mail.errors = []error{&resend.HTTPError{StatusCode: 429, RetryAfter: durationPtr(0)}}
	now := time.Now().UTC()
	require.NoError(t, workerFor(f, Options{Now: func() time.Time { return now }}).RunOnce(context.Background()))
	var next time.Time
	require.NoError(t, f.db.Pool.QueryRow(context.Background(), `SELECT next_attempt_at FROM report_notification_outbox WHERE idempotency_key = $1`, f.job.IdempotencyKey).Scan(&next))
	assert.WithinDuration(t, now, next, time.Second)
}

func TestWorker_PermanentErrorMarksSkipped(t *testing.T) {
	f := newWorkerFixture(t, "approved")
	f.mail.errors = []error{&resend.HTTPError{StatusCode: 422}}
	require.NoError(t, workerFor(f, Options{}).RunOnce(context.Background()))
	assert.Equal(t, "skipped", stateFor(t, f.db, f.job.IdempotencyKey))
}

func TestWorker_MaxAttemptsMarksSkipped(t *testing.T) {
	f := newWorkerFixture(t, "approved")
	f.mail.errors = []error{errors.New("temporary failure")}
	require.NoError(t, workerFor(f, Options{MaxAttempts: 1}).RunOnce(context.Background()))
	assert.Equal(t, "skipped", stateFor(t, f.db, f.job.IdempotencyKey))
}

func TestWorker_FailedRowDoesNotStopLaterRows(t *testing.T) {
	f := newWorkerFixture(t, "approved")
	second := f.job
	second.IdempotencyKey = "worker:2:1:123"
	require.NoError(t, outbox.NewStore(f.db).Enqueue(context.Background(), second))
	f.mail.errors = []error{errors.New("temporary failure"), nil}
	require.NoError(t, workerFor(f, Options{MaxAttempts: 2}).RunOnce(context.Background()))
	assert.Len(t, f.mail.messages, 2)
}

func TestWorker_RendererUsesOutboxSnapshot(t *testing.T) {
	f := newWorkerFixture(t, "needs_revision")
	require.NoError(t, workerFor(f, Options{}).RunOnce(context.Background()))
	require.Len(t, f.mail.messages, 1)
	assert.Contains(t, f.mail.messages[0].PlainText, "April 2026")
	assert.Contains(t, f.mail.messages[0].PlainText, "Please clarify")
	assert.Equal(t, "Your report needs revision", f.mail.messages[0].Subject)
}

func TestWorker_CancellationStopsPolling(t *testing.T) {
	f := newWorkerFixture(t, "approved")
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	require.NoError(t, workerFor(f, Options{}).Run(ctx))
	assert.Empty(t, f.mail.messages)
}

func TestWorker_InFlightFailureIsBounded(t *testing.T) {
	f := newWorkerFixture(t, "approved")
	f.mail.errors = []error{context.DeadlineExceeded}
	start := time.Now()
	require.NoError(t, workerFor(f, Options{InFlightTimeout: time.Millisecond, MaxAttempts: 2}).RunOnce(context.Background()))
	assert.Less(t, time.Since(start), time.Second)
}

func durationPtr(d time.Duration) *time.Duration { return &d }
