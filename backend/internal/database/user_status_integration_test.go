//go:build integration

package database_test

import (
	"context"
	"testing"

	"github.com/jackc/pgx/v5/pgconn"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestUsersStatus_DefaultsToActive(t *testing.T) {
	tdb := migratedDB(t)
	id := insertUser(t, tdb.Pool, "a@example.com")

	var status string
	require.NoError(t, tdb.Pool.QueryRow(context.Background(), `SELECT status FROM users WHERE id = $1`, id).Scan(&status))

	assert.Equal(t, "active", status)
}

func TestUsersStatus_RejectsAnUnknownValue(t *testing.T) {
	tdb := migratedDB(t)
	id := insertUser(t, tdb.Pool, "a@example.com")

	_, err := tdb.Pool.Exec(context.Background(), `UPDATE users SET status = 'retired' WHERE id = $1`, id)

	var pgErr *pgconn.PgError
	require.ErrorAs(t, err, &pgErr)
	assert.Equal(t, "23514", pgErr.Code)
	assert.Equal(t, "chk_users_status", pgErr.ConstraintName)
}

func TestAuditLogs_AcceptsChangeUserStatus(t *testing.T) {
	tdb := migratedDB(t)

	_, err := tdb.Pool.Exec(context.Background(),
		`INSERT INTO audit_logs (action, target_type) VALUES ('CHANGE_USER_STATUS', 'User')`)

	require.NoError(t, err, "without the CHECK change this audit row, and so the status change, would fail")
}
