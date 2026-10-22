//go:build integration

package auditimpl

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ssr0016/pastoral-reports/backend/internal/audit"
	"github.com/ssr0016/pastoral-reports/backend/internal/database"
	"github.com/ssr0016/pastoral-reports/backend/internal/testutil"
	"github.com/ssr0016/pastoral-reports/backend/pkg/pagination"
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

	t.Run("details key outside the allow-list writes nothing", func(t *testing.T) {
		var before, after int
		require.NoError(t, db.Pool.QueryRow(ctx, `SELECT count(*) FROM audit_logs`).Scan(&before))
		e := entry
		e.Details = map[string]any{"month": "January", "new_password": "hunter22"}
		require.ErrorIs(t, repo.Insert(ctx, e), audit.ErrDetailKey)
		require.NoError(t, db.Pool.QueryRow(ctx, `SELECT count(*) FROM audit_logs`).Scan(&after))
		assert.Equal(t, before, after)
	})

	t.Run("unknown action is rejected", func(t *testing.T) {
		e := entry
		e.Action = "NOPE"
		require.Error(t, repo.Insert(ctx, e))
	})
}

func TestAuditStore_List(t *testing.T) {
	tdb := testutil.SetupPostgres(t)
	tdb.MigrateWithGoose(t)
	db := &database.DB{Pool: tdb.Pool, Builder: database.NewStatementBuilder()}
	reader := NewReader(db)
	ctx := context.Background()
	manila, err := time.LoadLocation("Asia/Manila")
	require.NoError(t, err)

	// put writes a row at a chosen time; Insert always stamps NOW().
	put := func(action, target string, at time.Time) {
		t.Helper()
		_, err := tdb.Pool.Exec(ctx, `INSERT INTO audit_logs (action, target_type, target_name, ip_address, created_at)
			VALUES ($1, 'User', $2, '10.0.0.1', $3)`, action, target, at)
		require.NoError(t, err)
	}
	sep30Late := time.Date(2026, 9, 30, 23, 30, 0, 0, manila)
	oct1Early := time.Date(2026, 10, 1, 0, 30, 0, 0, manila) // 30 September in UTC
	put(audit.ActionLogin, "sep-late", sep30Late)
	put(audit.ActionLogin, "oct-early", oct1Early)
	tie := time.Date(2026, 10, 15, 9, 0, 0, 0, manila)
	for _, n := range []string{"tie-a", "tie-b", "tie-c"} {
		put(audit.ActionLoginFailed, n, tie)
	}
	octFrom := time.Date(2026, 10, 1, 0, 0, 0, 0, manila)
	octTo := octFrom.AddDate(0, 1, 0)
	names := func(rows []audit.Log) []string {
		out := []string{}
		for _, r := range rows {
			out = append(out, *r.TargetName)
		}
		return out
	}

	t.Run("the month is counted in Manila time", func(t *testing.T) {
		rows, total, err := reader.List(ctx, octFrom, octTo, nil, pagination.Params{Page: 1, Limit: 50})
		require.NoError(t, err)
		assert.EqualValues(t, 4, total)
		assert.Equal(t, []string{"tie-c", "tie-b", "tie-a", "oct-early"}, names(rows), "newest first, id breaks ties")

		sep, total, err := reader.List(ctx, octFrom.AddDate(0, -1, 0), octFrom, nil, pagination.Params{Page: 1, Limit: 50})
		require.NoError(t, err)
		assert.EqualValues(t, 1, total)
		assert.Equal(t, []string{"sep-late"}, names(sep))
	})

	t.Run("pages neither repeat nor skip a row on a tie", func(t *testing.T) {
		var seen []string
		for page := 1; page <= 4; page++ {
			rows, _, err := reader.List(ctx, octFrom, octTo, nil, pagination.Params{Page: page, Limit: 1})
			require.NoError(t, err)
			seen = append(seen, names(rows)...)
		}
		assert.Equal(t, []string{"tie-c", "tie-b", "tie-a", "oct-early"}, seen)
	})

	t.Run("action filter narrows rows and total", func(t *testing.T) {
		action := audit.ActionLogin
		rows, total, err := reader.List(ctx, octFrom, octTo, &action, pagination.Params{Page: 1, Limit: 50})
		require.NoError(t, err)
		assert.EqualValues(t, 1, total)
		assert.Equal(t, []string{"oct-early"}, names(rows))
	})

	t.Run("the IP address is never read", func(t *testing.T) {
		rows, _, err := reader.List(ctx, octFrom, octTo, nil, pagination.Params{Page: 1, Limit: 50})
		require.NoError(t, err)
		for _, r := range rows {
			assert.Nil(t, r.IPAddress)
		}
	})

	t.Run("details and empty columns scan", func(t *testing.T) {
		require.NoError(t, NewStore(db).Insert(ctx, audit.Log{Action: audit.ActionLoginFailed, TargetType: audit.TargetTypeUser,
			Details: map[string]any{"reason": audit.ReasonUnknownAccount}}))
		action := audit.ActionLoginFailed
		now := time.Now()
		rows, _, err := reader.List(ctx, now.Add(-time.Hour), now.Add(time.Hour), &action, pagination.Params{Page: 1, Limit: 50})
		require.NoError(t, err)
		require.Len(t, rows, 1)
		assert.Nil(t, rows[0].TargetID)
		assert.Nil(t, rows[0].PerformedBy)
		assert.Equal(t, map[string]any{"reason": audit.ReasonUnknownAccount}, rows[0].Details)
	})
}
