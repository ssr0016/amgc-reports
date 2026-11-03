package outbox

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"

	"github.com/ssr0016/pastoral-reports/backend/internal/database"
)

var ErrClaimLost = errors.New("notification claim lost")

type Job struct {
	ID             int64
	IdempotencyKey string
	EventType      string
	ReportID       int64
	RecipientEmail string
	ReportMonth    string
	ReportYear     int
	AdminRemark    string
	Attempts       int
	ClaimedAt      time.Time
}

type Queue interface {
	ClaimNext(ctx context.Context, now time.Time, leaseTimeout time.Duration) (*Job, error)
	MarkSent(ctx context.Context, id int64, claimedAt, sentAt time.Time) error
	MarkFailed(ctx context.Context, id int64, claimedAt, nextAttemptAt time.Time, lastError string) error
	MarkSkipped(ctx context.Context, id int64, claimedAt time.Time, reason string) error
}

func NewQueue(db *database.DB) Queue { return &Store{db: db} }

func (s *Store) ClaimNext(ctx context.Context, now time.Time, leaseTimeout time.Duration) (*Job, error) {
	var job *Job
	err := s.db.InTransaction(ctx, func(txCtx context.Context) error {
		var candidate Job
		var adminRemark *string
		err := s.db.Querier(txCtx).QueryRow(txCtx, `
			WITH candidate AS (
				SELECT id
				FROM report_notification_outbox
				WHERE
					((state IN ('pending', 'failed') AND next_attempt_at <= $1)
					 OR (state = 'claimed' AND claimed_at IS NOT NULL AND claimed_at <= $2))
				ORDER BY next_attempt_at, id
				FOR UPDATE SKIP LOCKED
				LIMIT 1
			)
			UPDATE report_notification_outbox AS o
			SET state = 'claimed', attempts = o.attempts + 1,
				claimed_at = $1, updated_at = $1
			FROM candidate
			WHERE o.id = candidate.id
				RETURNING o.id, o.idempotency_key, o.event_type, o.report_id,
				o.recipient_email, o.report_month, o.report_year,
				o.admin_remark, o.attempts, o.claimed_at
		`, now, now.Add(-leaseTimeout)).Scan(
			&candidate.ID, &candidate.IdempotencyKey, &candidate.EventType, &candidate.ReportID,
			&candidate.RecipientEmail, &candidate.ReportMonth, &candidate.ReportYear,
			&adminRemark, &candidate.Attempts, &candidate.ClaimedAt,
		)
		if errors.Is(err, pgx.ErrNoRows) {
			return nil
		}
		if err != nil {
			return fmt.Errorf("claim notification: %w", err)
		}
		if adminRemark != nil {
			candidate.AdminRemark = *adminRemark
		}
		job = &candidate
		return nil
	})
	return job, err
}

func (s *Store) MarkSent(ctx context.Context, id int64, claimedAt, sentAt time.Time) error {
	return s.mark(ctx, `
		UPDATE report_notification_outbox
		SET state = 'sent', sent_at = $3, claimed_at = NULL,
			last_error = NULL, updated_at = NOW()
		WHERE id = $1 AND state = 'claimed' AND claimed_at = $2
	`, id, claimedAt, sentAt)
}

func (s *Store) MarkFailed(ctx context.Context, id int64, claimedAt, nextAttemptAt time.Time, lastError string) error {
	return s.mark(ctx, `
		UPDATE report_notification_outbox
		SET state = 'failed', next_attempt_at = $3, claimed_at = NULL,
			last_error = $4, updated_at = NOW()
		WHERE id = $1 AND state = 'claimed' AND claimed_at = $2
	`, id, claimedAt, nextAttemptAt, lastError)
}

func (s *Store) MarkSkipped(ctx context.Context, id int64, claimedAt time.Time, reason string) error {
	return s.mark(ctx, `
		UPDATE report_notification_outbox
		SET state = 'skipped', skipped_at = NOW(), claimed_at = NULL,
			skip_reason = $3, updated_at = NOW()
		WHERE id = $1 AND state = 'claimed' AND claimed_at = $2
	`, id, claimedAt, reason)
}

func (s *Store) mark(ctx context.Context, query string, args ...any) error {
	tag, err := s.db.Querier(ctx).Exec(ctx, query, args...)
	if err != nil {
		return fmt.Errorf("update notification state: %w", err)
	}
	if tag.RowsAffected() != 1 {
		return ErrClaimLost
	}
	return nil
}
