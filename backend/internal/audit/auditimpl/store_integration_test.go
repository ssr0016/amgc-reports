//go:build integration

package auditimpl

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ssr0016/pastoral-reports/backend/internal/audit"
	"github.com/ssr0016/pastoral-reports/backend/internal/database"
	"github.com/ssr0016/pastoral-reports/backend/internal/testutil"
)

func TestAuditRepo_Insert(t *testing.T) {
	tdb := testutil.SetupPostgres(t)
	tdb.MigrateWithGoose(t)
	db := &database.DB{Pool: tdb.Pool, Builder: database.NewStatementBuilder()}
	repo := NewStore(db)
	ctx := context.Background()

	id, name, role, ip := "7", "Pastor A", "pastor", "10.0.0.1"
	entry := audit.Log{
		PerformedByName: &name, PerformedByRole: &role,
		Action: audit.ActionApproveReport, TargetType: audit.TargetTypeReport,
		TargetID: &id, IPAddress: &ip,
		Details: map[string]any{"month": "January", "year": 2026, "count": 1},
	}

	t.Run("commits with the transaction", func(t *testing.T) {
		require.NoError(t, db.InTransaction(ctx, func(ctx context.Context) error { return repo.Insert(ctx, entry) }))

		var gotType, gotMonth string
		require.NoError(t, db.Pool.QueryRow(ctx,
			`SELECT target_type, details->>'month' FROM audit_logs WHERE target_id = '7'`).Scan(&gotType, &gotMonth))
		assert.Equal(t, "Report", gotType)
		assert.Equal(t, "January", gotMonth)
	})

	t.Run("nil details stored as empty object", func(t *testing.T) {
		e := entry
		e.Details = nil
		require.NoError(t, repo.Insert(ctx, e))
		var n int
		require.NoError(t, db.Pool.QueryRow(ctx, `SELECT count(*) FROM audit_logs WHERE details = '{}'::jsonb`).Scan(&n))
		assert.Equal(t, 1, n)
	})

	t.Run("rolls back with the transaction", func(t *testing.T) {
		var before, after int
		require.NoError(t, db.Pool.QueryRow(ctx, `SELECT count(*) FROM audit_logs`).Scan(&before))
		_ = db.InTransaction(ctx, func(ctx context.Context) error {
			require.NoError(t, repo.Insert(ctx, entry))
			return assert.AnError
		})
		require.NoError(t, db.Pool.QueryRow(ctx, `SELECT count(*) FROM audit_logs`).Scan(&after))
		assert.Equal(t, before, after)
	})

	t.Run("unknown action is rejected", func(t *testing.T) {
		e := entry
		e.Action = "NOPE"
		require.Error(t, repo.Insert(ctx, e))
	})
}
