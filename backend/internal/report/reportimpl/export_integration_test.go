//go:build integration

package reportimpl

import (
	"bytes"
	"context"
	"errors"
	"strconv"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/xuri/excelize/v2"

	"github.com/ssr0016/pastoral-reports/backend/internal/audit"
	"github.com/ssr0016/pastoral-reports/backend/internal/audit/auditimpl"
	"github.com/ssr0016/pastoral-reports/backend/internal/database"
	"github.com/ssr0016/pastoral-reports/backend/internal/report"
	"github.com/ssr0016/pastoral-reports/backend/internal/testutil"
	"github.com/ssr0016/pastoral-reports/backend/internal/user/userimpl"
)

type failingAudit struct{}

func (failingAudit) Insert(context.Context, audit.Log) error {
	return errors.New("audit insert failed")
}

// Exports write exactly one audit row each, include reports of deleted and
// deceased owners, and leave soft-deleted reports out (D-A6, D-10).
func TestReportService_Export_Integration(t *testing.T) {
	tdb := testutil.SetupPostgres(t)
	tdb.MigrateWithGoose(t)
	db := &database.DB{Pool: tdb.Pool, Builder: database.NewStatementBuilder()}
	ctx := context.Background()
	users := userimpl.NewStore(db)
	reports := NewStore(db)

	admin, err := users.CreateWithPassword(ctx, "export-admin@example.com", "Admin", "hash")
	require.NoError(t, err)
	roles := stubRoles{perms: map[int64][]string{admin.RoleID: {"reports:read", "reports:read_any"}}}
	svc := NewService(reports, auditimpl.NewStore(db), db, users, roles)
	failing := NewService(reports, failingAudit{}, db, users, roles)

	file := func(email, church string) *report.Report {
		t.Helper()
		u, err := users.CreateWithPassword(ctx, email, church, "hash")
		require.NoError(t, err)
		rep, err := reports.Create(ctx, u.ID, 2026, report.CreateReportRequest{Month: "May", ChurchName: church,
			Metrics: map[string]report.MetricWeeks{"worship_service": {Week1: num("12")}}})
		require.NoError(t, err)
		return rep
	}
	zion := file("zion@example.com", "Zion")
	bethel := file("bethel@example.com", "Bethel")
	gone := file("gone@example.com", "Gone")
	require.NoError(t, reports.SoftDelete(ctx, gone.ID))
	_, err = tdb.Pool.Exec(ctx, "UPDATE users SET status = 'deceased' WHERE id = $1", bethel.CreatedBy)
	require.NoError(t, err)
	_, err = users.SoftDelete(ctx, bethel.CreatedBy, admin.ID, "private reason")
	require.NoError(t, err)

	type row struct {
		targetID *string
		details  string
	}
	rows := func(action string) []row {
		t.Helper()
		r, err := tdb.Pool.Query(ctx, `SELECT target_id, details::text FROM audit_logs WHERE action = $1 ORDER BY id`, action)
		require.NoError(t, err)
		defer r.Close()
		var out []row
		for r.Next() {
			var x row
			require.NoError(t, r.Scan(&x.targetID, &x.details))
			out = append(out, x)
		}
		require.NoError(t, r.Err())
		return out
	}

	t.Run("bulk", func(t *testing.T) {
		x, err := svc.ExportMonth(ctx, admin.ID, "10.0.0.9", report.ExportReportsRequest{Month: "May", Year: 2026})
		require.NoError(t, err)
		wb, err := excelize.OpenReader(bytes.NewReader(x.Data))
		require.NoError(t, err)
		defer func() { _ = wb.Close() }()
		assert.Equal(t, []string{"Bethel", "Zion"}, wb.GetSheetList())
		v, err := wb.GetCellValue("Zion", "B10")
		require.NoError(t, err)
		assert.Equal(t, "12", v)

		got := rows(audit.ActionDownloadBulk)
		require.Len(t, got, 1)
		assert.Nil(t, got[0].targetID)
		assert.JSONEq(t, `{"month":"May","year":2026,"count":2}`, got[0].details)
	})

	t.Run("single", func(t *testing.T) {
		_, err := svc.ExportOne(ctx, admin.ID, "", zion.ID)
		require.NoError(t, err)
		got := rows(audit.ActionDownloadSingle)
		require.Len(t, got, 1)
		assert.Equal(t, strconv.FormatInt(zion.ID, 10), *got[0].targetID)
		assert.JSONEq(t, `{"month":"May","year":2026,"count":1}`, got[0].details)

		_, err = svc.ExportOne(ctx, admin.ID, "", gone.ID)
		require.ErrorIs(t, err, report.ErrReportNotFound)
	})

	t.Run("failures write no audit row", func(t *testing.T) {
		_, err := failing.ExportOne(ctx, admin.ID, "", zion.ID)
		require.Error(t, err)
		_, err = failing.ExportMonth(ctx, admin.ID, "", report.ExportReportsRequest{Month: "May", Year: 2026})
		require.Error(t, err)
		_, err = svc.ExportMonth(ctx, admin.ID, "", report.ExportReportsRequest{Month: "June", Year: 2026})
		require.ErrorIs(t, err, report.ErrNoReports)
		assert.Len(t, rows(audit.ActionDownloadSingle), 1)
		assert.Len(t, rows(audit.ActionDownloadBulk), 1)
	})
}
