//go:build integration

package outbox

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestQueue_ClaimNextClaimsDuePending(t *testing.T) {
	f := newOutboxFixture(t)
	n := notificationFor(f)
	require.NoError(t, f.store.Enqueue(context.Background(), n))

	q := NewQueue(f.db)
	now := time.Now().UTC()
	job, err := q.ClaimNext(context.Background(), now, time.Minute)
	require.NoError(t, err)
	require.NotNil(t, job)
	assert.Equal(t, n.ReportID, job.ReportID)
	assert.Equal(t, 1, job.Attempts)
}

func TestQueue_ClaimNextHandlesNullAdminRemark(t *testing.T) {
	f := newOutboxFixture(t)
	_, err := f.db.Pool.Exec(context.Background(), `
		INSERT INTO report_notification_outbox (
			idempotency_key, event_type, report_id, recipient_user_id,
			recipient_email, report_month, report_year, admin_remark
		) VALUES ($1, 'approved', $2, $3, $4, 'April', 2026, NULL)
	`, "approved:null-remark", f.report.ID, f.recipient, "first@example.com")
	require.NoError(t, err)

	job, err := NewQueue(f.db).ClaimNext(context.Background(), time.Now().UTC(), time.Minute)
	require.NoError(t, err)
	require.NotNil(t, job)
	assert.Equal(t, "", job.AdminRemark)
}

func TestQueue_ClaimNextReturnsNilWhenNoRowDue(t *testing.T) {
	f := newOutboxFixture(t)
	n := notificationFor(f)
	require.NoError(t, f.store.Enqueue(context.Background(), n))
	_, err := f.db.Pool.Exec(context.Background(), `UPDATE report_notification_outbox SET next_attempt_at = NOW() + INTERVAL '1 hour' WHERE idempotency_key = $1`, n.IdempotencyKey)
	require.NoError(t, err)

	job, err := NewQueue(f.db).ClaimNext(context.Background(), time.Now().UTC(), time.Minute)
	require.NoError(t, err)
	assert.Nil(t, job)
}

func TestQueue_ClaimNextRecoversStaleClaim(t *testing.T) {
	f := newOutboxFixture(t)
	n := notificationFor(f)
	require.NoError(t, f.store.Enqueue(context.Background(), n))
	q := NewQueue(f.db)
	first, err := q.ClaimNext(context.Background(), time.Now().UTC(), time.Minute)
	require.NoError(t, err)
	require.NotNil(t, first)
	_, err = f.db.Pool.Exec(context.Background(), `UPDATE report_notification_outbox SET claimed_at = NOW() - INTERVAL '2 hours' WHERE idempotency_key = $1`, n.IdempotencyKey)
	require.NoError(t, err)

	second, err := q.ClaimNext(context.Background(), time.Now().UTC(), time.Minute)
	require.NoError(t, err)
	require.NotNil(t, second)
	assert.Equal(t, 2, second.Attempts)
}

func TestQueue_MarkSentUpdatesMatchingClaim(t *testing.T) {
	f := newOutboxFixture(t)
	n := notificationFor(f)
	require.NoError(t, f.store.Enqueue(context.Background(), n))
	q := NewQueue(f.db)
	job, err := q.ClaimNext(context.Background(), time.Now().UTC(), time.Minute)
	require.NoError(t, err)
	require.NoError(t, q.MarkSent(context.Background(), job.ID, job.ClaimedAt, time.Now().UTC()))

	var state string
	require.NoError(t, f.db.Pool.QueryRow(context.Background(), `SELECT state FROM report_notification_outbox WHERE id = $1`, job.ID).Scan(&state))
	assert.Equal(t, "sent", state)
}

func TestQueue_MarkFailedAndSkippedUpdateMatchingClaim(t *testing.T) {
	for _, tt := range []struct {
		name string
		mark func(Queue, *Job) error
		want string
	}{
		{name: "failed", mark: func(q Queue, j *Job) error {
			return q.MarkFailed(context.Background(), j.ID, j.ClaimedAt, time.Now().Add(time.Minute), "temporary")
		}, want: "failed"},
		{name: "skipped", mark: func(q Queue, j *Job) error {
			return q.MarkSkipped(context.Background(), j.ID, j.ClaimedAt, "permanent")
		}, want: "skipped"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			f := newOutboxFixture(t)
			n := notificationFor(f)
			require.NoError(t, f.store.Enqueue(context.Background(), n))
			q := NewQueue(f.db)
			job, err := q.ClaimNext(context.Background(), time.Now().UTC(), time.Minute)
			require.NoError(t, err)
			require.NoError(t, tt.mark(q, job))
			var state string
			require.NoError(t, f.db.Pool.QueryRow(context.Background(), `SELECT state FROM report_notification_outbox WHERE id = $1`, job.ID).Scan(&state))
			assert.Equal(t, tt.want, state)
		})
	}
}

func TestQueue_ClaimLostGuardProtectsReclaimedRow(t *testing.T) {
	f := newOutboxFixture(t)
	n := notificationFor(f)
	require.NoError(t, f.store.Enqueue(context.Background(), n))
	q := NewQueue(f.db)
	old, err := q.ClaimNext(context.Background(), time.Now().UTC(), time.Minute)
	require.NoError(t, err)
	_, err = f.db.Pool.Exec(context.Background(), `UPDATE report_notification_outbox SET claimed_at = NOW() - INTERVAL '2 hours' WHERE id = $1`, old.ID)
	require.NoError(t, err)
	current, err := q.ClaimNext(context.Background(), time.Now().UTC(), time.Minute)
	require.NoError(t, err)
	require.ErrorIs(t, q.MarkSent(context.Background(), old.ID, old.ClaimedAt, time.Now().UTC()), ErrClaimLost)
	require.NoError(t, q.MarkSent(context.Background(), current.ID, current.ClaimedAt, time.Now().UTC()))
}

func TestQueue_ClaimNextUsesSkipLocked(t *testing.T) {
	f := newOutboxFixture(t)
	n := notificationFor(f)
	require.NoError(t, f.store.Enqueue(context.Background(), n))
	q := NewQueue(f.db)
	start := make(chan struct{})
	results := make(chan *Job, 2)
	errs := make(chan error, 2)
	var wg sync.WaitGroup
	for i := 0; i < 2; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			<-start
			job, err := q.ClaimNext(context.Background(), time.Now().UTC(), time.Minute)
			results <- job
			errs <- err
		}()
	}
	close(start)
	wg.Wait()
	close(results)
	close(errs)
	var claimed int
	for job := range results {
		if job != nil {
			claimed++
		}
	}
	for err := range errs {
		require.NoError(t, err)
	}
	assert.Equal(t, 1, claimed)
}
