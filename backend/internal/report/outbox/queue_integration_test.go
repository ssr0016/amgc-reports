//go:build integration

package outbox

import (
	"context"
	"fmt"
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
	require.NotNil(t, job.ReportID)
	assert.Equal(t, n.ReportID, *job.ReportID)
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

func TestQueue_ClaimNextHandlesNullReportID(t *testing.T) {
	f := newOutboxFixture(t)
	_, err := f.db.Pool.Exec(context.Background(), `
		INSERT INTO report_notification_outbox (
			idempotency_key, event_type, report_id, recipient_user_id,
			recipient_email, report_month, report_year
		) VALUES ($1, 'daily_digest', NULL, $2, $3, 'April', 2026)
	`, "daily-digest:null-report", f.recipient, "first@example.com")
	require.NoError(t, err)

	job, err := NewQueue(f.db).ClaimNext(context.Background(), time.Now().UTC(), time.Minute)
	require.NoError(t, err)
	require.NotNil(t, job)
	assert.Nil(t, job.ReportID)
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

func TestStore_ScheduleDailyDigestIsIdempotentAndCarriesPayload(t *testing.T) {
	f := newOutboxFixture(t)
	ctx := context.Background()
	_, err := f.db.Pool.Exec(ctx, `INSERT INTO roles (name, description) VALUES ('admin', 'Admin') ON CONFLICT (name) DO NOTHING`)
	require.NoError(t, err)
	_, err = f.db.Pool.Exec(ctx, `INSERT INTO permissions (name, resource, action) VALUES ('reports:approve', 'reports', 'approve') ON CONFLICT (name) DO NOTHING`)
	require.NoError(t, err)
	var adminID int64
	require.NoError(t, f.db.Pool.QueryRow(ctx, `
		INSERT INTO users (email, name, password_hash, role_id, notification_prefs)
		VALUES ('digest-admin@example.com', 'Digest Admin', 'hash', (SELECT id FROM roles WHERE name = 'admin'), NULL)
		RETURNING id`).Scan(&adminID))
	_, err = f.db.Pool.Exec(ctx, `
		INSERT INTO role_permissions (role_id, permission_id)
		SELECT (SELECT id FROM roles WHERE name = 'admin'), id FROM permissions WHERE name = 'reports:approve'
		ON CONFLICT DO NOTHING`)
	require.NoError(t, err)

	day := time.Date(2026, time.November, 3, 8, 5, 0, 0, time.FixedZone("Asia/Manila", 8*60*60))
	s := NewStore(f.db)
	require.NoError(t, s.ScheduleDailyDigest(ctx, day, "https://reports.example.com/"))
	require.NoError(t, s.ScheduleDailyDigest(ctx, day, "https://reports.example.com/"))
	require.NoError(t, s.ScheduleDailyDigest(ctx, day.Add(time.Minute), "https://reports.example.com/"))

	var count int
	var payload []byte
	var keys []string
	require.NoError(t, f.db.Pool.QueryRow(ctx, `SELECT COUNT(*) FROM report_notification_outbox WHERE recipient_user_id = $1`, adminID).Scan(&count))
	assert.Equal(t, 1, count)
	rows, err := f.db.Pool.Query(ctx, `SELECT idempotency_key FROM report_notification_outbox WHERE recipient_user_id = $1 ORDER BY idempotency_key`, adminID)
	require.NoError(t, err)
	for rows.Next() {
		var key string
		require.NoError(t, rows.Scan(&key))
		keys = append(keys, key)
	}
	rows.Close()
	assert.Equal(t, []string{"daily-digest:2026-11-03:" + fmt.Sprint(adminID)}, keys)
	require.NoError(t, f.db.Pool.QueryRow(ctx, `SELECT payload FROM report_notification_outbox WHERE recipient_user_id = $1`, adminID).Scan(&payload))
	assert.Contains(t, string(payload), `"reports_url": "https://reports.example.com/admin?tab=reports"`)
}
