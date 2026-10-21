//go:build integration

package repository

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ssr0016/pastoral-reports/backend/internal/database"
	"github.com/ssr0016/pastoral-reports/backend/internal/model"
	"github.com/ssr0016/pastoral-reports/backend/internal/testutil"
)

func TestStatsRepo_Counts(t *testing.T) {
	tdb := testutil.SetupPostgres(t)
	tdb.MigrateWithGoose(t)
	db := &database.DB{Pool: tdb.Pool, Builder: database.NewStatementBuilder()}
	stats := NewStatsRepo(db)
	reports := NewReportRepo(db)
	users := NewUserRepo(db)
	ctx := context.Background()

	t.Run("empty database is all zeros", func(t *testing.T) {
		got, err := stats.Counts(ctx, "January", 2026)
		require.NoError(t, err)
		assert.Equal(t, model.AdminStats{Month: "January", Year: 2026}, got)
	})

	// The migrations seed only the "user" role; "admin" is seed data, so the test adds it.
	adminRole, err := NewRoleRepo(db).Create(ctx, model.CreateRoleRequest{Name: "admin"})
	require.NoError(t, err)
	ana, err := users.CreateWithPassword(ctx, "ana@example.com", "Ana", "hash")
	require.NoError(t, err)
	bob, err := users.CreateWithPassword(ctx, "bob@example.com", "Bob", "hash")
	require.NoError(t, err)
	boss, err := users.CreateWithPassword(ctx, "boss@example.com", "Boss", "hash")
	require.NoError(t, err)
	require.NoError(t, users.UpdateRole(ctx, boss.ID, adminRole.ID))

	file := func(userID int64, month string, year int) *model.Report {
		t.Helper()
		rep, err := reports.Create(ctx, userID, year, model.CreateReportRequest{Month: month, NarrativeReport: "n"})
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
		got, err := stats.Counts(ctx, "January", 2026)
		require.NoError(t, err)
		// janAna approved, Bob pending; janBoss is soft-deleted.
		assert.EqualValues(t, 2, got.ReportsTotal)
		assert.EqualValues(t, 1, got.ReportsApproved)
		assert.EqualValues(t, 1, got.ReportsPending)
		assert.Equal(t, got.ReportsTotal, got.ReportsApproved+got.ReportsPending)
	})

	t.Run("an approval taken back counts as pending", func(t *testing.T) {
		_, err := reports.ToggleComplete(ctx, janAna.ID, boss.ID)
		require.NoError(t, err)
		got, err := stats.Counts(ctx, "January", 2026)
		require.NoError(t, err)
		assert.EqualValues(t, 0, got.ReportsApproved)
		assert.EqualValues(t, 2, got.ReportsPending)
	})

	t.Run("month matches case-insensitively and echoes the arguments", func(t *testing.T) {
		got, err := stats.Counts(ctx, "january", 2026)
		require.NoError(t, err)
		assert.EqualValues(t, 2, got.ReportsTotal)
		assert.Equal(t, 2026, got.Year)
	})

	t.Run("a period with no reports is zero but users are still counted", func(t *testing.T) {
		got, err := stats.Counts(ctx, "March", 2030)
		require.NoError(t, err)
		assert.EqualValues(t, 0, got.ReportsTotal)
		assert.EqualValues(t, 3, got.UsersTotal)
	})

	t.Run("admins are counted by role name, users are all accounts", func(t *testing.T) {
		got, err := stats.Counts(ctx, "January", 2026)
		require.NoError(t, err)
		assert.EqualValues(t, 3, got.UsersTotal)
		assert.EqualValues(t, 1, got.AdminsTotal)
	})
}
