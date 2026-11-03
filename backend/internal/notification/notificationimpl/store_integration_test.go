//go:build integration

package notificationimpl

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ssr0016/pastoral-reports/backend/internal/database"
	"github.com/ssr0016/pastoral-reports/backend/internal/notification"
	"github.com/ssr0016/pastoral-reports/backend/internal/testutil"
)

func TestStore_CreateReportSubmitted_SelectsActiveApprovers(t *testing.T) {
	tdb := testutil.SetupPostgres(t)
	tdb.MigrateWithGoose(t)
	db, err := database.NewWithURL(context.Background(), tdb.DSN)
	require.NoError(t, err)
	t.Cleanup(func() { db.Pool.Close() })
	ctx := context.Background()

	_, err = tdb.Pool.Exec(ctx, `
		INSERT INTO roles (name, description) VALUES ('reviewer', 'Reviewer')
		ON CONFLICT (name) DO NOTHING;
		INSERT INTO permissions (name, resource, action)
		VALUES ('reports:approve', 'reports', 'approve')
		ON CONFLICT (name) DO NOTHING;
		INSERT INTO role_permissions (role_id, permission_id)
		SELECT r.id, p.id FROM roles r, permissions p
		WHERE r.name = 'reviewer' AND p.name = 'reports:approve'
		ON CONFLICT DO NOTHING`)
	require.NoError(t, err)

	userID := insertNotificationUser(t, tdb, "approver", "reviewer", "active", false)
	noPermissionID := insertNotificationUser(t, tdb, "plain", "user", "active", false)
	inactiveID := insertNotificationUser(t, tdb, "inactive", "reviewer", "inactive", false)
	deletedID := insertNotificationUser(t, tdb, "deleted", "reviewer", "active", true)

	store := NewStore(db)
	require.NoError(t, store.CreateReportSubmitted(ctx, 42, "Report submitted by Ana"))

	rows, err := tdb.Pool.Query(ctx, `
		SELECT user_id, type, message, metadata->>'report_id'
		FROM user_notifications ORDER BY user_id`)
	require.NoError(t, err)
	defer rows.Close()

	var got []struct {
		userID  int64
		typ     string
		message string
		report  string
	}
	for rows.Next() {
		var row struct {
			userID  int64
			typ     string
			message string
			report  string
		}
		require.NoError(t, rows.Scan(&row.userID, &row.typ, &row.message, &row.report))
		got = append(got, row)
	}
	require.NoError(t, rows.Err())
	require.Len(t, got, 1)
	assert.Equal(t, userID, got[0].userID)
	assert.Equal(t, "report_submitted", got[0].typ)
	assert.Equal(t, "Report submitted by Ana", got[0].message)
	assert.Equal(t, "42", got[0].report)
	assert.NotEqual(t, noPermissionID, got[0].userID)
	assert.NotEqual(t, inactiveID, got[0].userID)
	assert.NotEqual(t, deletedID, got[0].userID)
}

func TestStore_ListUnreadAndMarkRead_AreUserScoped(t *testing.T) {
	tdb := testutil.SetupPostgres(t)
	tdb.MigrateWithGoose(t)
	db, err := database.NewWithURL(context.Background(), tdb.DSN)
	require.NoError(t, err)
	t.Cleanup(func() { db.Pool.Close() })
	ctx := context.Background()
	userID := insertNotificationUser(t, tdb, "reader", "user", "active", false)
	otherID := insertNotificationUser(t, tdb, "other-reader", "user", "active", false)

	ids := make([]int64, 0, 3)
	for _, message := range []string{"old", "new", "already read"} {
		var id int64
		require.NoError(t, tdb.Pool.QueryRow(ctx, `
			INSERT INTO user_notifications (user_id, type, message, metadata, created_at)
			VALUES ($1, 'report_submitted', $2, $3, NOW() + ($4 * INTERVAL '1 second'))
			RETURNING id`, userID, message, `{"report_id": 7}`, len(ids)).Scan(&id))
		ids = append(ids, id)
	}
	_, err = tdb.Pool.Exec(ctx, `UPDATE user_notifications SET read_at = NOW() WHERE id = $1`, ids[2])
	require.NoError(t, err)
	var otherNotificationID int64
	require.NoError(t, tdb.Pool.QueryRow(ctx, `
		INSERT INTO user_notifications (user_id, type, message)
		VALUES ($1, 'report_submitted', 'other') RETURNING id`, otherID).Scan(&otherNotificationID))

	store := NewStore(db)
	got, err := store.ListUnread(ctx, userID)
	require.NoError(t, err)
	require.Len(t, got, 2)
	assert.Equal(t, "new", got[0].Message)
	assert.Equal(t, "old", got[1].Message)
	assert.Equal(t, int64(7), int64(got[0].Metadata["report_id"].(float64)))

	require.NoError(t, store.MarkRead(ctx, userID, got[0].ID))
	_, err = store.ListUnread(ctx, userID)
	require.NoError(t, err)
	assert.ErrorIs(t, store.MarkRead(ctx, userID, got[0].ID), notification.ErrNotFound)
	assert.ErrorIs(t, store.MarkRead(ctx, userID, otherNotificationID), notification.ErrNotFound)

	var readAt *time.Time
	require.NoError(t, tdb.Pool.QueryRow(ctx, `SELECT read_at FROM user_notifications WHERE id = $1`, got[0].ID).Scan(&readAt))
	assert.NotNil(t, readAt)

}

func insertNotificationUser(t *testing.T, tdb *testutil.TestDB, name, role, status string, deleted bool) int64 {
	t.Helper()
	var id int64
	var deletedAt *time.Time
	if deleted {
		now := time.Now()
		deletedAt = &now
	}
	require.NoError(t, tdb.Pool.QueryRow(context.Background(), `
		INSERT INTO users (email, name, password_hash, role_id, status, deleted_at)
		VALUES ($1, $2, 'hash', (SELECT id FROM roles WHERE name = $3), $4, $5)
		RETURNING id`, name+"@example.com", name, role, status, deletedAt).Scan(&id))
	return id
}
