package notificationimpl

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	sq "github.com/Masterminds/squirrel"
	"github.com/jackc/pgx/v5"

	"github.com/ssr0016/pastoral-reports/backend/internal/database"
	"github.com/ssr0016/pastoral-reports/backend/internal/notification"
)

type store struct{ db *database.DB }

func NewStore(db *database.DB) notification.Store { return &store{db: db} }

const notificationColumns = "id, user_id, type, message, metadata, read_at, created_at"

func scanNotification(row pgx.Row) (notification.Notification, error) {
	var n notification.Notification
	var raw []byte
	if err := row.Scan(&n.ID, &n.UserID, &n.Type, &n.Message, &raw, &n.ReadAt, &n.CreatedAt); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return notification.Notification{}, notification.ErrNotFound
		}
		return notification.Notification{}, fmt.Errorf("scan notification: %w", err)
	}
	if err := json.Unmarshal(raw, &n.Metadata); err != nil {
		return notification.Notification{}, fmt.Errorf("decode notification metadata: %w", err)
	}
	return n, nil
}

func (s *store) CreateReportSubmitted(ctx context.Context, reportID int64, message string) error {
	const query = `
		INSERT INTO user_notifications (user_id, type, message, metadata)
		SELECT u.id, 'report_submitted', $1, jsonb_build_object('report_id', $2::bigint)
		FROM users u
		JOIN roles r ON r.id = u.role_id
		JOIN role_permissions rp ON rp.role_id = r.id
		JOIN permissions p ON p.id = rp.permission_id
		WHERE p.name = 'reports:approve'
		  AND u.status = 'active'
		  AND u.deleted_at IS NULL`
	if _, err := s.db.Querier(ctx).Exec(ctx, query, message, reportID); err != nil {
		return fmt.Errorf("create report notifications: %w", err)
	}
	return nil
}

func (s *store) ListUnread(ctx context.Context, userID int64) ([]notification.Notification, error) {
	query, args, err := s.db.Builder.
		Select(notificationColumns).
		From("user_notifications").
		Where(sq.Eq{"user_id": userID, "read_at": nil}).
		OrderBy("created_at DESC", "id DESC").
		Limit(50).
		ToSql()
	if err != nil {
		return nil, fmt.Errorf("build list notifications query: %w", err)
	}
	rows, err := s.db.Querier(ctx).Query(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("list notifications: %w", err)
	}
	defer rows.Close()

	out := make([]notification.Notification, 0)
	for rows.Next() {
		n, err := scanNotification(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, n)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("read notifications: %w", err)
	}
	return out, nil
}

func (s *store) MarkRead(ctx context.Context, userID, id int64) error {
	query, args, err := s.db.Builder.
		Update("user_notifications").
		Set("read_at", sq.Expr("NOW()")).
		Where(sq.Eq{"id": id, "user_id": userID, "read_at": nil}).
		ToSql()
	if err != nil {
		return fmt.Errorf("build mark notification query: %w", err)
	}
	result, err := s.db.Querier(ctx).Exec(ctx, query, args...)
	if err != nil {
		return fmt.Errorf("mark notification read: %w", err)
	}
	if result.RowsAffected() == 0 {
		return notification.ErrNotFound
	}
	return nil
}
