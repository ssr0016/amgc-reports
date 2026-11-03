//go:build integration

package outbox

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ssr0016/pastoral-reports/backend/internal/database"
	"github.com/ssr0016/pastoral-reports/backend/internal/report"
	"github.com/ssr0016/pastoral-reports/backend/internal/report/reportimpl"
	"github.com/ssr0016/pastoral-reports/backend/internal/testutil"
	"github.com/ssr0016/pastoral-reports/backend/internal/user/userimpl"
)

type outboxFixture struct {
	db        *database.DB
	store     report.NotificationSink
	report    *report.Report
	recipient int64
}

func newOutboxFixture(t *testing.T) outboxFixture {
	t.Helper()
	tdb := testutil.SetupPostgres(t)
	tdb.MigrateWithGoose(t)
	db := &database.DB{Pool: tdb.Pool, Builder: database.NewStatementBuilder()}
	ctx := context.Background()
	users := userimpl.NewStore(db)
	reports := reportimpl.NewStore(db)
	owner, err := users.CreateWithPassword(ctx, "outbox-owner@example.com", "Owner", "hash")
	require.NoError(t, err)
	rep, err := reports.Create(ctx, owner.ID, 2026, report.CreateReportRequest{Month: "April"})
	require.NoError(t, err)
	return outboxFixture{db: db, store: NewStore(db), report: rep, recipient: owner.ID}
}

func notificationFor(f outboxFixture) report.Notification {
	return report.Notification{
		EventType:       "approved",
		IdempotencyKey:  "approved:1:1234567890",
		ReportID:        f.report.ID,
		RecipientUserID: f.recipient,
		RecipientEmail:  "first@example.com",
		Month:           "April",
		Year:            2026,
		AdminRemark:     "first remark",
	}
}

func TestStore_Enqueue(t *testing.T) {
	f := newOutboxFixture(t)
	ctx := context.Background()
	n := notificationFor(f)

	require.NoError(t, f.store.Enqueue(ctx, n))

	var state string
	var nextAttemptAt time.Time
	var email, remark string
	err := f.db.Pool.QueryRow(ctx, `
		SELECT state, next_attempt_at, recipient_email, admin_remark
		FROM report_notification_outbox
		WHERE idempotency_key = $1
	`, n.IdempotencyKey).Scan(&state, &nextAttemptAt, &email, &remark)
	require.NoError(t, err)
	assert.Equal(t, "pending", state)
	assert.LessOrEqual(t, time.Since(nextAttemptAt).Abs(), time.Second)
	assert.Equal(t, n.RecipientEmail, email)
	assert.Equal(t, n.AdminRemark, remark)
}

func TestStore_Enqueue_IdempotencyConflictIsIgnored(t *testing.T) {
	f := newOutboxFixture(t)
	ctx := context.Background()
	first := notificationFor(f)
	second := first
	second.RecipientEmail = "second@example.com"
	second.AdminRemark = "second remark"

	require.NoError(t, f.store.Enqueue(ctx, first))
	require.NoError(t, f.store.Enqueue(ctx, second))

	var count int
	err := f.db.Pool.QueryRow(ctx, `
		SELECT COUNT(*) FROM report_notification_outbox
		WHERE idempotency_key = $1
	`, first.IdempotencyKey).Scan(&count)
	require.NoError(t, err)
	assert.Equal(t, 1, count)

	var email, remark string
	err = f.db.Pool.QueryRow(ctx, `
		SELECT recipient_email, admin_remark
		FROM report_notification_outbox
		WHERE idempotency_key = $1
	`, first.IdempotencyKey).Scan(&email, &remark)
	require.NoError(t, err)
	assert.Equal(t, first.RecipientEmail, email)
	assert.Equal(t, first.AdminRemark, remark)
}
