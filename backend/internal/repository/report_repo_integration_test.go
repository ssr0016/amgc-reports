//go:build integration

package repository

import (
	"context"
	"encoding/json"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ssr0016/pastoral-reports/backend/internal/database"
	"github.com/ssr0016/pastoral-reports/backend/internal/model"
	"github.com/ssr0016/pastoral-reports/backend/internal/testutil"
	"github.com/ssr0016/pastoral-reports/backend/pkg/pagination"
)

func num(s string) *json.Number { n := json.Number(s); return &n }

func TestReportRepo(t *testing.T) {
	tdb := testutil.SetupPostgres(t)
	tdb.MigrateWithGoose(t)
	db := &database.DB{Pool: tdb.Pool, Builder: database.NewStatementBuilder()}
	repo := NewReportRepo(db)
	ctx := context.Background()

	user, err := NewUserRepo(db).CreateWithPassword(ctx, "rep@example.com", "Rep", "hash")
	require.NoError(t, err)

	mk := func(month, worker string, metrics map[string]model.MetricWeeks) model.CreateReportRequest {
		return model.CreateReportRequest{Month: month, Worker: worker, NarrativeReport: "n", Metrics: metrics}
	}
	create := func(req model.CreateReportRequest, year int) *model.Report {
		t.Helper()
		rep, err := repo.Create(ctx, db.Pool, user.ID, year, req)
		require.NoError(t, err)
		return rep
	}

	jan := create(mk("January", "Ana_100%", map[string]model.MetricWeeks{
		"tithes_offering": {Week1: num("120.00"), Week3: num("0.5")},
	}), 2026)

	t.Run("numeric round-trips as text, absent weeks omitted", func(t *testing.T) {
		got, err := repo.Get(ctx, db.Pool, jan.ID)
		require.NoError(t, err)
		w := got.ToResponse().Metrics["tithes_offering"]
		require.NotNil(t, w.Week1)
		assert.Equal(t, "120.00", string(*w.Week1))
		assert.Equal(t, "0.50", string(*w.Week3))
		assert.Nil(t, w.Week2)
	})

	t.Run("duplicate month/year is ErrUniqueViolation", func(t *testing.T) {
		_, err := repo.Create(ctx, db.Pool, user.ID, 2026, mk("January", "", nil))
		require.ErrorIs(t, err, ErrUniqueViolation)
	})

	t.Run("list filters escape wildcards and load metrics for the page", func(t *testing.T) {
		create(mk("February", "Bob", map[string]model.MetricWeeks{"outreach": {Week1: num("3")}}), 2026)
		pg := pagination.Params{Page: 1, Limit: 10}

		got, total, err := repo.List(ctx, model.ListReportsParams{Worker: "100%"}, pg)
		require.NoError(t, err)
		assert.EqualValues(t, 1, total)
		require.Len(t, got, 1)
		assert.Equal(t, jan.ID, got[0].ID)
		assert.Contains(t, got[0].Metrics, "tithes_offering")

		// "_" must not act as a single-char wildcard: "A_a" would match "Ana" if unescaped
		_, total, err = repo.List(ctx, model.ListReportsParams{Worker: "A_a"}, pg)
		require.NoError(t, err)
		assert.EqualValues(t, 0, total)

		got, total, err = repo.List(ctx, model.ListReportsParams{Month: "FEBRUARY", Year: 2026}, pg)
		require.NoError(t, err)
		assert.EqualValues(t, 1, total)
		assert.Contains(t, got[0].Metrics, "outreach")

		got, total, err = repo.List(ctx, model.ListReportsParams{}, pagination.Params{Page: 2, Limit: 1})
		require.NoError(t, err)
		assert.EqualValues(t, 2, total)
		assert.Len(t, got, 1)
	})

	t.Run("list can be limited to one owner", func(t *testing.T) {
		other, err := NewUserRepo(db).CreateWithPassword(ctx, "other@example.com", "Other", "hash")
		require.NoError(t, err)
		_, err = repo.Create(ctx, db.Pool, other.ID, 2026, mk("March", "", nil))
		require.NoError(t, err)

		got, total, err := repo.List(ctx, model.ListReportsParams{CreatedBy: other.ID}, pagination.Params{Page: 1, Limit: 10})
		require.NoError(t, err)
		assert.EqualValues(t, 1, total)
		assert.Equal(t, other.ID, got[0].CreatedBy)
	})

	t.Run("update replaces metrics and keeps completed fields", func(t *testing.T) {
		upd := model.UpdateReportRequest(mk("January", "Ana", map[string]model.MetricWeeks{"outreach": {Week2: num("7.25")}}))
		got, err := repo.Update(ctx, db.Pool, jan.ID, 2026, upd)
		require.NoError(t, err)
		assert.NotContains(t, got.Metrics, "tithes_offering")
		assert.Equal(t, "7.25", string(*got.ToResponse().Metrics["outreach"].Week2))

		_, err = repo.Update(ctx, db.Pool, 99999, 2026, upd)
		require.ErrorIs(t, err, ErrNotFound)

		// moving onto another active (month, year) collides
		upd.Month = "February"
		_, err = repo.Update(ctx, db.Pool, jan.ID, 2026, upd)
		require.ErrorIs(t, err, ErrUniqueViolation)
	})

	t.Run("toggle approves then unapproves", func(t *testing.T) {
		on, err := repo.ToggleComplete(ctx, db.Pool, jan.ID, user.ID)
		require.NoError(t, err)
		require.True(t, on.Completed)
		require.NotNil(t, on.CompletedAt)
		assert.Equal(t, &user.ID, on.CompletedBy)

		off, err := repo.ToggleComplete(ctx, db.Pool, jan.ID, user.ID)
		require.NoError(t, err)
		assert.False(t, off.Completed)
		assert.Nil(t, off.CompletedAt)
		assert.Nil(t, off.CompletedBy)
	})

	t.Run("soft-deleted report is invisible everywhere and its period is reusable", func(t *testing.T) {
		require.NoError(t, repo.SoftDelete(ctx, db.Pool, jan.ID))

		got, err := repo.Get(ctx, db.Pool, jan.ID)
		require.NoError(t, err)
		assert.Nil(t, got)

		_, total, err := repo.List(ctx, model.ListReportsParams{Month: "January"}, pagination.Params{Page: 1, Limit: 10})
		require.NoError(t, err)
		assert.EqualValues(t, 0, total)

		require.ErrorIs(t, repo.SoftDelete(ctx, db.Pool, jan.ID), ErrNotFound)
		_, err = repo.ToggleComplete(ctx, db.Pool, jan.ID, user.ID)
		require.ErrorIs(t, err, ErrNotFound)
		_, err = repo.Update(ctx, db.Pool, jan.ID, 2026, model.UpdateReportRequest(mk("January", "", nil)))
		require.ErrorIs(t, err, ErrNotFound)

		_, err = repo.Create(ctx, db.Pool, user.ID, 2026, mk("January", "", nil))
		require.NoError(t, err)
	})
}

// The partial unique index (created_by, month, year) WHERE deleted_at IS NULL is
// the guarantee: of N simultaneous creates for one period, exactly one wins.
func TestReportRepo_ConcurrentCreateSamePeriod(t *testing.T) {
	tdb := testutil.SetupPostgres(t)
	tdb.MigrateWithGoose(t)
	db := &database.DB{Pool: tdb.Pool, Builder: database.NewStatementBuilder()}
	repo := NewReportRepo(db)
	ctx := context.Background()

	user, err := NewUserRepo(db).CreateWithPassword(ctx, "race@example.com", "Race", "hash")
	require.NoError(t, err)

	const n = 8
	req := model.CreateReportRequest{Month: "March", Metrics: map[string]model.MetricWeeks{"outreach": {Week1: num("1")}}}
	start := make(chan struct{})
	errs := make(chan error, n)
	for i := 0; i < n; i++ {
		go func() {
			<-start
			_, err := repo.Create(ctx, db.Pool, user.ID, 2026, req)
			errs <- err
		}()
	}
	close(start)

	var ok, dup int
	for i := 0; i < n; i++ {
		switch err := <-errs; {
		case err == nil:
			ok++
		case errors.Is(err, ErrUniqueViolation):
			dup++
		default:
			t.Errorf("unexpected error: %v", err)
		}
	}
	assert.Equal(t, 1, ok, "exactly one create wins")
	assert.Equal(t, n-1, dup, "the rest are ErrUniqueViolation")

	var reports, metrics int
	require.NoError(t, db.Pool.QueryRow(ctx, `SELECT count(*) FROM reports WHERE created_by = $1`, user.ID).Scan(&reports))
	require.NoError(t, db.Pool.QueryRow(ctx, `SELECT count(*) FROM report_metrics`).Scan(&metrics))
	assert.Equal(t, 1, reports)
	assert.Equal(t, 1, metrics, "losers insert no metrics")
}
