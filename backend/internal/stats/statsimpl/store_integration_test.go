//go:build integration

package statsimpl

import (
	"context"
	"encoding/json"
	"testing"
	"time"

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

func TestStore_Analytics(t *testing.T) {
	tdb := testutil.SetupPostgres(t)
	tdb.MigrateWithGoose(t)
	db := &database.DB{Pool: tdb.Pool, Builder: database.NewStatementBuilder()}
	store := NewStore(db)
	reports := reportimpl.NewStore(db)
	users := userimpl.NewStore(db)
	ctx := context.Background()
	// account creates an active user-role account for church; the trigger adds the church row.
	account := func(email, church string) int64 {
		t.Helper()
		u, err := users.CreateWithPassword(ctx, email, church, "hash")
		require.NoError(t, err)
		_, err = users.UpdateProfile(ctx, u.ID, user.UserProfile{Name: church + " Worker", ChurchName: church})
		require.NoError(t, err)
		return u.ID
	}
	bethel := account("analytics@example.com", "Bethel")
	account("zion@example.com", " zion ")
	account("zion2@example.com", "Zion") // same church, second account
	hope := account("hope@example.com", "Hope")
	_, err := reports.Create(ctx, hope, 2026, report.CreateReportRequest{Month: "April"}) // outside Q1
	require.NoError(t, err)
	create := func(month string, year int, metrics map[string]report.MetricWeeks) int64 {
		t.Helper()
		r, err := reports.Create(ctx, bethel, year, report.CreateReportRequest{Month: month, Metrics: metrics})
		require.NoError(t, err)
		return r.ID
	}
	jan := create("January", 2026, map[string]report.MetricWeeks{
		"worship_service": {Week1: analyticsNum("2"), Week3: analyticsNum("0")},
		"tithes_offering": {Week1: analyticsNum("120.50")},
	})
	create("March", 2026, map[string]report.MetricWeeks{"worship_service": {Week1: analyticsNum("7")}})
	create("January", 2025, map[string]report.MetricWeeks{"worship_service": {Week1: analyticsNum("3")}})
	gone := create("February", 2026, nil)
	_, err = tdb.Pool.Exec(ctx, "UPDATE reports SET deleted_at = NOW() WHERE id = $1", gone)
	require.NoError(t, err)

	churches, err := store.Churches(ctx)
	require.NoError(t, err)
	require.Len(t, churches, 3, "one row per church name, however it is spaced or cased")
	names := map[string]stats.ChurchRecord{}
	// A new church starts at the month being filed now, in Manila (the previous month).
	now := time.Now().In(manila)
	open := time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, time.UTC).AddDate(0, -1, 0)
	for _, c := range churches {
		names[c.Name] = c
		require.NotNil(t, c.StartMonth, c.Name)
		assert.Equal(t, open.Format(time.DateOnly), c.StartMonth.Format(time.DateOnly), c.Name)
	}
	def, err := store.ReportingDefault(ctx)
	require.NoError(t, err)
	require.NotNil(t, def)
	assert.Equal(t, "2026-09-01", def.Format(time.DateOnly), "the organization default confirmed by the owner")
	assert.Len(t, names["zion"].Accounts, 2)
	assert.Equal(t, "Bethel Worker", names["Bethel"].Accounts[0].Name)

	got, err := store.AnalyticsReports(ctx, []int{2026, 2025}, 1, 3)
	require.NoError(t, err)
	require.Len(t, got, 3, "Q1 of both years; April and the deleted report left out")
	first := got[0]
	assert.Equal(t, jan, first.ID)
	assert.Equal(t, names["Bethel"].ID, first.ChurchID)
	assert.Equal(t, 1, first.Month)
	require.NotNil(t, first.Metrics["worship_service"])
	ws := first.Metrics["worship_service"]
	assert.Equal(t, 2.0, *ws[0])
	assert.Nil(t, ws[1], "a blank week stays nil")
	assert.Equal(t, 0.0, *ws[2], "an explicit zero is kept")
	assert.Equal(t, 120.50, *first.Metrics["tithes_offering"][0])

	t.Run("reporting months are stored on the church, in a transaction", func(t *testing.T) {
		id := names["Bethel"].ID
		start, end := month(2026, 9), month(2027, 2)
		var before, after stats.ChurchRecord
		require.NoError(t, db.InTransaction(ctx, func(ctx context.Context) error {
			var err error
			before, after, err = store.SetChurchReporting(ctx, id, start, end)
			return err
		}))
		assert.Equal(t, open.Format("2006-01"), before.StartMonth.Format("2006-01"))
		assert.Equal(t, "2026-09", after.StartMonth.Format("2006-01"))
		churches, err := store.Churches(ctx)
		require.NoError(t, err)
		for _, c := range churches {
			if c.ID == id {
				assert.Equal(t, "2026-09-01", c.StartMonth.Format(time.DateOnly))
				assert.Equal(t, "2027-02-01", c.EndMonth.Format(time.DateOnly))
			}
		}
		_, _, err = store.SetChurchReporting(ctx, 9999, start, nil)
		assert.ErrorIs(t, err, stats.ErrChurchNotFound)
		_, _, err = store.SetChurchReporting(ctx, id, end, start)
		assert.Error(t, err, "chk_churches_period refuses an end before the start")
	})
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
		// The migrations seed the "user" role, so it is listed with no accounts.
		require.Len(t, got.RoleCounts, 1)
		assert.Equal(t, "user", got.RoleCounts[0].Name)
		assert.Zero(t, got.RoleCounts[0].Count)
		assert.Equal(t, []stats.StatusCount{{Status: "active"}, {Status: "inactive"}, {Status: "disciplinary"}, {Status: "deceased"}}, got.StatusCounts)
		got.RoleCounts, got.StatusCounts = nil, nil
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
		assert.EqualValues(t, 3, got.UsersActive)
		assert.EqualValues(t, 1, got.AdminsTotal)
	})

	t.Run("every role and every status is counted, zeros kept and deleted accounts left out", func(t *testing.T) {
		_, err := acimpl.NewRoleStore(db).Create(ctx, accesscontrol.CreateRoleRequest{Name: "reviewer"})
		require.NoError(t, err)
		_, err = users.UpdateStatus(ctx, bob.ID, user.UserStatusInactive)
		require.NoError(t, err)
		got, err := store.Counts(ctx, "January", 2026)
		require.NoError(t, err)
		byRole := map[string]int64{}
		for _, r := range got.RoleCounts {
			byRole[r.Name] = r.Count
		}
		assert.Equal(t, map[string]int64{"user": 2, "admin": 1, "reviewer": 0}, byRole)
		assert.Equal(t, []stats.StatusCount{
			{Status: "active", Count: 2}, {Status: "inactive", Count: 1}, {Status: "disciplinary"}, {Status: "deceased"},
		}, got.StatusCounts)
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
		for i := range got {
			assert.False(t, got[i].CreatedAt.IsZero(), "created_at of %s", got[i].Name)
			got[i].CreatedAt = time.Time{}
		}
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
