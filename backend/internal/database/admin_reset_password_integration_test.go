//go:build integration

package database_test

import (
	"context"
	"testing"

	"github.com/jackc/pgx/v5/pgconn"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// An admin resetting another user's password is audited in the same transaction
// (D-A12). Until the CHECK allows the action, the whole reset would roll back.
func TestAuditLogs_AcceptsAdminResetPassword(t *testing.T) {
	tdb := migratedDB(t)

	_, err := tdb.Pool.Exec(context.Background(),
		`INSERT INTO audit_logs (action, target_type) VALUES ('ADMIN_RESET_PASSWORD', 'User')`)

	require.NoError(t, err, "without the CHECK change the reset, and so the endpoint, would fail with a 500")
}

func TestAuditLogs_StillRejectsAnUnknownAction(t *testing.T) {
	tdb := migratedDB(t)

	_, err := tdb.Pool.Exec(context.Background(),
		`INSERT INTO audit_logs (action, target_type) VALUES ('VIEW_PASSWORD', 'User')`)

	var pgErr *pgconn.PgError
	require.ErrorAs(t, err, &pgErr)
	assert.Equal(t, "23514", pgErr.Code)
	assert.Equal(t, "chk_audit_logs_action", pgErr.ConstraintName)
}
