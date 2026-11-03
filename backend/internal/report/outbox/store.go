package outbox

import (
	"context"
	"fmt"

	"github.com/ssr0016/pastoral-reports/backend/internal/database"
	"github.com/ssr0016/pastoral-reports/backend/internal/report"
)

type Store struct {
	db *database.DB
}

func NewStore(db *database.DB) report.NotificationSink {
	return &Store{db: db}
}

func (s *Store) Enqueue(ctx context.Context, n report.Notification) error {
	_, err := s.db.Querier(ctx).Exec(ctx, `
		INSERT INTO report_notification_outbox (
			idempotency_key,
			event_type,
			report_id,
			recipient_user_id,
			recipient_email,
			report_month,
			report_year,
			admin_remark
		)
		VALUES ($1, $2, $3, $4, $5, $6, $7, NULLIF($8, ''))
		ON CONFLICT (idempotency_key) DO NOTHING
	`, n.IdempotencyKey, n.EventType, n.ReportID, n.RecipientUserID,
		n.RecipientEmail, n.Month, n.Year, n.AdminRemark)
	if err != nil {
		return fmt.Errorf("enqueue report notification: %w", err)
	}
	return nil
}
