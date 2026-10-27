//go:build integration

package statsimpl

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ssr0016/pastoral-reports/backend/internal/accesscontrol"
	"github.com/ssr0016/pastoral-reports/backend/internal/accesscontrol/acimpl"
	"github.com/ssr0016/pastoral-reports/backend/internal/database"
	"github.com/ssr0016/pastoral-reports/backend/internal/report"
	"github.com/ssr0016/pastoral-reports/backend/internal/report/reportimpl"
	"github.com/ssr0016/pastoral-reports/backend/internal/stats"
	"github.com/ssr0016/pastoral-reports/backend/internal/testutil"
	"github.com/ssr0016/pastoral-reports/backend/internal/user"
	"github.com/ssr0016/pastoral-reports/backend/internal/user/userimpl"
)

func analyticsNum(s string) *json.Number { n := json.Number(s); return &n }

func TestStore_AnalyticsAttendance(t *testing.T) {
	tdb := testutil.SetupPostgres(t)
	tdb.MigrateWithGoose(t)
	db := &database.DB{Pool: tdb.Pool, Builder: database.NewStatementBuilder()}
	store := NewStore(db)
	reports := reportimpl.NewStore(db)
	worker, err := userimpl.NewStore(db).CreateWithPassword(context.Background(), "analytics@example.com", "Analytics", "hash")
	require.NoError(t, err)
	ctx := context.Background()
	create := func(month string, year int, value string) {
		t.Helper()
		metrics := map[string]report.MetricWeeks{"worship_service": {Week1: analyticsNum(value)}}
		if month == "January" && year == 2026 {
			metrics["tithes_offering"] = report.MetricWeeks{Week1: analyticsNum("120.50")}
			metrics["baptism"] = report.MetricWeeks{Week1: analyticsNum("0")}
		}
		_, err := reports.Create(ctx, worker.ID, year, report.CreateReportRequest{
			Month:   month,
			Metrics: metrics,
		})
		require.NoError(t, err)
	}
	create("January", 2026, "2")
	create("April", 2026, "9")
	create("January", 2025, "3")

	got, err := store.AnalyticsAttendance(ctx, stats.AttendanceQuery{
		Year: 2026, Metric: "worship_service", Status: stats.AttendanceStatusAll,
		Period: "q1", Compare: true,
	})
	require.NoError(t, err)
	require.Len(t, got.Trend, 15)
	require.Len(t, got.Churches, 1)
	assert.Equal(t, 2.0, got.Churches[0].Value)
	require.Len(t, got.Summary, 21)
	tithes := got.Summary[8]
	assert.Equal(t, "tithes_offering", tithes.Metric)
	require.NotNil(t, tithes.Total)
	assert.Equal(t, 120.50, *tithes.Total)
	require.Len(t, tithes.Churches, 1)
	require.NotNil(t, tithes.Churches[0].Value)
	assert.Equal(t, 120.50, *tithes.Churches[0].Value)
	baptism := got.Summary[18]
	require.NotNil(t, baptism.Total)
	assert.Equal(t, 0.0, *baptism.Total)
	sundaySchool := got.Summary[1]
	assert.Nil(t, sundaySchool.Total)
	require.Len(t, sundaySchool.Churches, 1)
	assert.Nil(t, sundaySchool.Churches[0].Value)
	require.NotNil(t, got.Comparison)
	assert.Equal(t, 2025, got.Comparison.Year)
	assert.Len(t, got.Comparison.Trend, 15)
	require.Len(t, got.Comparison.Churches, 1)
	assert.Equal(t, 3.0, got.Comparison.Churches[0].Value)
}

func TestStore_Counts(t *testing.T) {
	tdb := testutil.SetupPostgres(t)
	tdb.MigrateWithGoose(t)
	db := &database.DB{Pool: tdb.Pool, Builder: database.NewStatementBuilder()}
	store := NewStore(db)
	reports := reportimpl.NewStore(db)
	users := userimpl.NewStore(db)
	ctx := context.Background()

	t.Run("empty database is all zeros", func(t *testing.T) {
		got, err := store.Counts(ctx, "January", 2026)
		require.NoError(t, err)
		assert.Equal(t, stats.AdminStats{Month: "January", Year: 2026}, got)
	})

	// The migrations seed only the "user" role; "admin" is seed data, so the test adds it.
	adminRole, err := acimpl.NewRoleStore(db).Create(ctx, accesscontrol.CreateRoleRequest{Name: "admin"})
	require.NoError(t, err)
	ana, err := users.CreateWithPassword(ctx, "ana@example.com", "Ana", "hash")
	require.NoError(t, err)
	bob, err := users.CreateWithPassword(ctx, "bob@example.com", "Bob", "hash")
	require.NoError(t, err)
	require.NoError(t, func() error {
		_, err := users.UpdateProfile(ctx, ana.ID, user.UserProfile{Name: "Ana", ChurchName: "Bethel"})
		return err
	}())
	require.NoError(t, func() error {
		_, err := users.UpdateProfile(ctx, bob.ID, user.UserProfile{Name: "Bob", ChurchName: "Ephesus"})
		return err
	}())
	boss, err := users.CreateWithPassword(ctx, "boss@example.com", "Boss", "hash")
	require.NoError(t, err)
	require.NoError(t, users.UpdateRole(ctx, boss.ID, adminRole.ID))
	// Soft-deleted accounts, one of them an admin, are not counted (D-A24).
	gone, err := users.CreateWithPassword(ctx, "gone@example.com", "Gone", "hash")
	require.NoError(t, err)
	exBoss, err := users.CreateWithPassword(ctx, "exboss@example.com", "Ex Boss", "hash")
	require.NoError(t, err)
	require.NoError(t, users.UpdateRole(ctx, exBoss.ID, adminRole.ID))
	_, err = users.SoftDelete(ctx, gone.ID, boss.ID, "left")
	require.NoError(t, err)
	_, err = users.SoftDelete(ctx, exBoss.ID, boss.ID, "left")
	require.NoError(t, err)

	file := func(userID int64, month string, year int) *report.Report {
		t.Helper()
		rep, err := reports.Create(ctx, userID, year, report.CreateReportRequest{Month: month, NarrativeReport: "n"})
		require.NoError(t, err)
		return rep
	}
	janAna := file(ana.ID, "January", 2026)
	file(bob.ID, "January", 2026)
	janBoss := file(boss.ID, "January", 2026)
	file(ana.ID, "February", 2026) // another month
	file(ana.ID, "January", 2025)  // another year
	_, err = reports.ToggleComplete(ctx, janAna.ID, boss.ID)
	require.NoError(t, err)
	require.NoError(t, reports.SoftDelete(ctx, janBoss.ID))

	t.Run("counts the period, not other months, years or deleted reports", func(t *testing.T) {
		got, err := store.Counts(ctx, "January", 2026)
		require.NoError(t, err)
		// janAna approved, Bob pending; janBoss is soft-deleted.
		assert.EqualValues(t, 2, got.ReportsTotal)
		assert.EqualValues(t, 1, got.ReportsApproved)
		assert.EqualValues(t, 1, got.ReportsPending)
		assert.EqualValues(t, 2, got.ExpectedReports)
		assert.EqualValues(t, 0, got.NoReport)
		assert.EqualValues(t, 2, got.ChurchesTotal)
		assert.Equal(t, got.ReportsTotal, got.ReportsApproved+got.ReportsPending)
	})

	t.Run("an approval taken back counts as pending", func(t *testing.T) {
		_, err := reports.ToggleComplete(ctx, janAna.ID, boss.ID)
		require.NoError(t, err)
		got, err := store.Counts(ctx, "January", 2026)
		require.NoError(t, err)
		assert.EqualValues(t, 0, got.ReportsApproved)
		assert.EqualValues(t, 1, got.ReportsPending)
		assert.EqualValues(t, 1, got.ReportsNeedsRevision)
	})

	t.Run("month matches case-insensitively and echoes the arguments", func(t *testing.T) {
		got, err := store.Counts(ctx, "january", 2026)
		require.NoError(t, err)
		assert.EqualValues(t, 2, got.ReportsTotal)
		assert.Equal(t, 2026, got.Year)
	})

	t.Run("a period with no reports is zero but users are still counted", func(t *testing.T) {
		got, err := store.Counts(ctx, "March", 2030)
		require.NoError(t, err)
		assert.EqualValues(t, 0, got.ReportsTotal)
		assert.EqualValues(t, 3, got.UsersTotal)
		assert.EqualValues(t, 2, got.ExpectedReports)
		assert.EqualValues(t, 2, got.NoReport)
		assert.EqualValues(t, 2, got.ChurchesTotal)
	})

	t.Run("admins are counted by role name, users are all accounts, deleted ones aside", func(t *testing.T) {
		got, err := store.Counts(ctx, "January", 2026)
		require.NoError(t, err)
		assert.EqualValues(t, 3, got.UsersTotal)
		assert.EqualValues(t, 1, got.AdminsTotal)
	})
}

func TestStore_Tracker(t *testing.T) {
	tdb := testutil.SetupPostgres(t)
	tdb.MigrateWithGoose(t)
	db := &database.DB{Pool: tdb.Pool, Builder: database.NewStatementBuilder()}
	store := NewStore(db)
	reports := reportimpl.NewStore(db)
	users := userimpl.NewStore(db)
	roles := acimpl.NewRoleStore(db)
	ctx := context.Background()

	t.Run("no accounts is an empty list, not nil", func(t *testing.T) {
		got, err := store.Tracker(ctx, "January", 2026)
		require.NoError(t, err)
		assert.NotNil(t, got)
		assert.Empty(t, got)
	})

	// account creates a user-role account with a church ("" leaves it NULL).
	account := func(email, name, church string) int64 {
		t.Helper()
		u, err := users.CreateWithPassword(ctx, email, name, "hash")
		require.NoError(t, err)
		if church != "" {
			_, err = users.UpdateProfile(ctx, u.ID, user.UserProfile{Name: name, ChurchName: church})
			require.NoError(t, err)
		}
		return u.ID
	}
	file := func(userID int64, month string, year int) *report.Report {
		t.Helper()
		rep, err := reports.Create(ctx, userID, year, report.CreateReportRequest{Month: month, NarrativeReport: "n"})
		require.NoError(t, err)
		return rep
	}

	adminRole, err := roles.Create(ctx, accesscontrol.CreateRoleRequest{Name: "admin"})
	require.NoError(t, err)
	editorRole, err := roles.Create(ctx, accesscontrol.CreateRoleRequest{Name: "editor"})
	require.NoError(t, err)

	ana := account("ana@example.com", "Ana", "Bethel")     // approved
	bob := account("bob@example.com", "Bob", "antioch")    // pending
	eva := account("eva@example.com", "Eva", "Ephesus")    // needs revision
	cora := account("cora@example.com", "Cora", "Antioch") // no report; inactive
	dan := account("dan@example.com", "Dan", "")           // no church, so not a church account
	gone := account("gone@example.com", "Gone", "Calvary") // soft-deleted account
	boss := account("boss@example.com", "Boss", "HQ")      // admin
	ed := account("ed@example.com", "Ed", "Berea")         // editor
	require.NoError(t, users.UpdateRole(ctx, boss, adminRole.ID))
	require.NoError(t, users.UpdateRole(ctx, ed, editorRole.ID))
	_, err = users.UpdateStatus(ctx, cora, "inactive")
	require.NoError(t, err)
	_, err = users.SoftDelete(ctx, gone, boss, "left")
	require.NoError(t, err)

	_, err = reports.ToggleComplete(ctx, file(ana, "January", 2026).ID, boss)
	require.NoError(t, err)
	file(bob, "January", 2026)
	_, err = reports.RequestRevision(ctx, file(eva, "January", 2026).ID, boss)
	require.NoError(t, err)
	require.NoError(t, reports.SoftDelete(ctx, file(dan, "January", 2026).ID))
	file(dan, "February", 2026)
	file(dan, "January", 2025)
	file(gone, "January", 2026)
	file(boss, "January", 2026)
	file(ed, "January", 2026)

	t.Run("church accounts only, with their status, by church then name", func(t *testing.T) {
		got, err := store.Tracker(ctx, "january", 2026)
		require.NoError(t, err)
		assert.Equal(t, []stats.TrackerRow{
			{UserID: bob, Name: "Bob", Email: "bob@example.com", ChurchName: "antioch", UserStatus: "active", SubmissionStatus: stats.TrackerSubmitted, ReviewStatus: stats.TrackerPending},
			{UserID: cora, Name: "Cora", Email: "cora@example.com", ChurchName: "Antioch", UserStatus: "inactive", SubmissionStatus: stats.TrackerNoReport},
			{UserID: ana, Name: "Ana", Email: "ana@example.com", ChurchName: "Bethel", UserStatus: "active", SubmissionStatus: stats.TrackerSubmitted, ReviewStatus: stats.TrackerApproved},
			{UserID: eva, Name: "Eva", Email: "eva@example.com", ChurchName: "Ephesus", UserStatus: "active", SubmissionStatus: stats.TrackerSubmitted, ReviewStatus: stats.TrackerNeedsRevision},
		}, got)
	})

	t.Run("a month with no reports lists everyone as not submitted", func(t *testing.T) {
		got, err := store.Tracker(ctx, "March", 2030)
		require.NoError(t, err)
		require.Len(t, got, 4)
		for _, row := range got {
			assert.Equal(t, stats.TrackerNoReport, row.SubmissionStatus, row.Name)
			assert.Empty(t, row.ReviewStatus, row.Name)
		}
	})
}
