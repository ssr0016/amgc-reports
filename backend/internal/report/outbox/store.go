package outbox

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/ssr0016/pastoral-reports/backend/internal/database"
	"github.com/ssr0016/pastoral-reports/backend/internal/report"
)

type Store struct {
	db *database.DB
}

func NewStore(db *database.DB) *Store {
	return &Store{db: db}
}

type DigestScheduler interface {
	ScheduleDailyDigest(ctx context.Context, day time.Time, reportsURL string) error
}

func (s *Store) ScheduleDailyDigest(ctx context.Context, day time.Time, reportsURL string) error {
	month, year := day.Format("January"), day.Year()
	reportsURL = strings.TrimRight(reportsURL, "/") + "/admin?tab=reports"
	_, err := s.db.Querier(ctx).Exec(ctx, `
		WITH counts AS (
			SELECT
				(SELECT COUNT(*) FROM reports WHERE deleted_at IS NULL AND year = $1 AND lower(month) = lower($2)
					AND status NOT IN ('approved', 'needs_revision')) AS pending_count,
				(SELECT COUNT(*) FROM users u JOIN roles r ON r.id = u.role_id
					WHERE r.name = 'user' AND u.status = 'active' AND u.deleted_at IS NULL
					AND btrim(COALESCE(u.church_name, '')) <> ''
					AND NOT EXISTS (SELECT 1 FROM reports rp WHERE rp.created_by = u.id AND rp.deleted_at IS NULL
						AND rp.year = $1 AND lower(rp.month) = lower($2))) AS missing_count
		)
		INSERT INTO report_notification_outbox
			(idempotency_key, event_type, recipient_user_id, recipient_email, report_month, report_year, payload)
		SELECT 'daily-digest:' || $3 || ':' || u.id, 'daily_digest', u.id, u.email, $2, $1,
			jsonb_build_object('month', $2, 'year', $1, 'pending_count', c.pending_count,
				'missing_count', c.missing_count, 'reports_url', $4::text)
		FROM users u JOIN roles r ON r.id = u.role_id
		JOIN role_permissions rpp ON rpp.role_id = r.id
		JOIN permissions p ON p.id = rpp.permission_id AND p.name = 'reports:approve'
		CROSS JOIN counts c
		WHERE r.name = 'admin' AND u.status = 'active' AND u.deleted_at IS NULL
			AND CASE WHEN u.notification_prefs->>'daily_digest' IN ('true', 'false')
				THEN (u.notification_prefs->>'daily_digest')::boolean ELSE true END
		ON CONFLICT (idempotency_key) DO NOTHING
	`, year, month, day.Format("2006-01-02"), reportsURL)
	if err != nil {
		return fmt.Errorf("schedule daily digest: %w", err)
	}
	return nil
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
