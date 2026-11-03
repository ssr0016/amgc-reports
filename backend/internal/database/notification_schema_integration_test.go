//go:build integration

package database_test

import (
	"context"
	"testing"

	"github.com/jackc/pgx/v5/pgconn"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ssr0016/pastoral-reports/backend/internal/testutil"
)

func TestUserNotificationsSchema_ObjectsExist(t *testing.T) {
	tdb := testutil.SetupPostgres(t)
	tdb.MigrateWithGoose(t)

	var exists bool
	require.NoError(t, tdb.Pool.QueryRow(context.Background(),
		`SELECT to_regclass('public.user_notifications') IS NOT NULL`).Scan(&exists))
	assert.True(t, exists)

	var indexExists bool
	require.NoError(t, tdb.Pool.QueryRow(context.Background(), `
		SELECT EXISTS (
			SELECT 1 FROM pg_indexes
			WHERE schemaname = 'public' AND indexname = 'idx_user_notifications_user_read_at'
		)`).Scan(&indexExists))
	assert.True(t, indexExists)

	rows, err := tdb.Pool.Query(context.Background(), `
		SELECT column_name, is_nullable, data_type
		FROM information_schema.columns
		WHERE table_schema = 'public' AND table_name = 'user_notifications'
		ORDER BY ordinal_position`)
	require.NoError(t, err)
	defer rows.Close()

	columns := map[string][2]string{}
	for rows.Next() {
		var name, nullable, dataType string
		require.NoError(t, rows.Scan(&name, &nullable, &dataType))
		columns[name] = [2]string{nullable, dataType}
	}
	require.NoError(t, rows.Err())

	assert.Equal(t, [2]string{"NO", "bigint"}, columns["id"])
	assert.Equal(t, [2]string{"NO", "bigint"}, columns["user_id"])
	assert.Equal(t, [2]string{"NO", "character varying"}, columns["type"])
	assert.Equal(t, [2]string{"NO", "text"}, columns["message"])
	assert.Equal(t, [2]string{"NO", "jsonb"}, columns["metadata"])
	assert.Equal(t, [2]string{"YES", "timestamp with time zone"}, columns["read_at"])
	assert.Equal(t, [2]string{"NO", "timestamp with time zone"}, columns["created_at"])
}

func TestUserNotificationsSchema_Constraints(t *testing.T) {
	tdb := testutil.SetupPostgres(t)
	tdb.MigrateWithGoose(t)
	ctx := context.Background()

	var userID int64
	require.NoError(t, tdb.Pool.QueryRow(ctx, `
		INSERT INTO users (email, name, password_hash, role_id)
		VALUES ('notifications-schema@example.com', 'Schema', 'hash',
			(SELECT id FROM roles WHERE name = 'user'))
		RETURNING id`).Scan(&userID))

	_, err := tdb.Pool.Exec(ctx, `
		INSERT INTO user_notifications (user_id, type, message, metadata)
		VALUES ($1, 'report_submitted', 'Report submitted', '{"report_id": 1}')`, userID)
	require.NoError(t, err)

	_, err = tdb.Pool.Exec(ctx, `
		INSERT INTO user_notifications (user_id, type, message)
		VALUES ($1, 'unknown', 'Bad type')`, userID)
	var pgErr *pgconn.PgError
	require.ErrorAs(t, err, &pgErr)
	assert.Equal(t, "23514", pgErr.Code)
	assert.Equal(t, "chk_user_notifications_type", pgErr.ConstraintName)

	_, err = tdb.Pool.Exec(ctx, `
		INSERT INTO user_notifications (user_id, type, message)
		VALUES (999999999, 'report_submitted', 'Bad user')`)
	require.ErrorAs(t, err, &pgErr)
	assert.Equal(t, "23503", pgErr.Code)
}
