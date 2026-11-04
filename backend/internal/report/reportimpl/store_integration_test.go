//go:build integration

package reportimpl

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ssr0016/pastoral-reports/backend/internal/audit"
	"github.com/ssr0016/pastoral-reports/backend/internal/audit/auditimpl"
	"github.com/ssr0016/pastoral-reports/backend/internal/database"
	"github.com/ssr0016/pastoral-reports/backend/internal/notification"
	"github.com/ssr0016/pastoral-reports/backend/internal/report"
	"github.com/ssr0016/pastoral-reports/backend/internal/testutil"
	"github.com/ssr0016/pastoral-reports/backend/internal/user"
	"github.com/ssr0016/pastoral-reports/backend/internal/user/userimpl"
	"github.com/ssr0016/pastoral-reports/backend/pkg/pagination"
)

func num(s string) *json.Number { n := json.Number(s); return &n }

type failingNotificationSink struct {
	db  *database.DB
	err error
}

type failingInAppStore struct{ err error }

func (s failingInAppStore) CreateReportSubmitted(context.Context, int64, string) error { return s.err }
func (failingInAppStore) ListUnread(context.Context, int64) ([]notification.Notification, error) {
	return nil, nil
}
func (failingInAppStore) MarkRead(context.Context, int64, int64) error { return nil }

func TestReportService_CreateNotificationFailureRollsBack(t *testing.T) {
	tdb := testutil.SetupPostgres(t)
	tdb.MigrateWithGoose(t)
	db := &database.DB{Pool: tdb.Pool, Builder: database.NewStatementBuilder()}
	ctx := context.Background()
	users := userimpl.NewStore(db)
	reports := NewStore(db)
	audits := auditimpl.NewStore(db)
	owner, err := users.CreateWithPassword(ctx, "create-notify-owner@example.com", "Owner", "hash")
	require.NoError(t, err)
	caller := stubbedCaller{Store: users, roles: stubRoles{perms: map[int64][]string{
		owner.RoleID: {"reports:read", "reports:write", "reports:delete"},
	}}}
	boom := errors.New("in-app notification insert failed")
	svc := NewService(reports, audits, db, caller, nil, failingInAppStore{err: boom}).(*service)
	svc.now = func() time.Time { return time.Date(2026, 5, 1, 0, 0, 0, 0, time.UTC) }

	_, err = svc.Create(ctx, owner.ID, "", report.CreateReportRequest{
		Month: "April", Metrics: map[string]report.MetricWeeks{"outreach": {Week1: num("3")}},
	})
	require.ErrorIs(t, err, boom)

	var count int
	for _, table := range []string{"reports", "report_metrics", "audit_logs", "user_notifications"} {
		require.NoError(t, tdb.Pool.QueryRow(ctx, fmt.Sprintf("SELECT COUNT(*) FROM %s", table)).Scan(&count))
		assert.Zero(t, count, table)
	}
}

func (s failingNotificationSink) Enqueue(ctx context.Context, n report.Notification) error {
	_, err := s.db.Querier(ctx).Exec(ctx, `
		INSERT INTO report_notification_outbox
			(idempotency_key, event_type, report_id, recipient_user_id,
			 recipient_email, report_month, report_year, admin_remark)
		VALUES ($1, $2, $3, $4, $5, $6, $7, NULLIF($8, ''))
	`, n.IdempotencyKey, n.EventType, n.ReportID, n.RecipientUserID,
		n.RecipientEmail, n.Month, n.Year, n.AdminRemark)
	if err != nil {
		return err
	}
	return s.err
}

func TestReportService_NotificationFailureAbortsTransition(t *testing.T) {
	tdb := testutil.SetupPostgres(t)
	tdb.MigrateWithGoose(t)
	db := &database.DB{Pool: tdb.Pool, Builder: database.NewStatementBuilder()}
	ctx := context.Background()
	users := userimpl.NewStore(db)
	reports := NewStore(db)
	audits := auditimpl.NewStore(db)

	owner, err := users.CreateWithPassword(ctx, "notify-owner@example.com", "Owner", "hash")
	require.NoError(t, err)
	admin, err := users.CreateWithPassword(ctx, "notify-admin@example.com", "Admin", "hash")
	require.NoError(t, err)
	roles := stubRoles{perms: map[int64][]string{
		owner.RoleID: {"reports:read", "reports:write", "reports:delete"},
		admin.RoleID: {"reports:read", "reports:read_any", "reports:approve"},
	}}
	caller := stubbedCaller{Store: users, roles: roles}
	created, err := reports.Create(ctx, owner.ID, 2026, report.CreateReportRequest{Month: "April"})
	require.NoError(t, err)

	boom := errors.New("outbox insert failed")
	svc := NewService(reports, audits, db, caller, failingNotificationSink{db: db, err: boom})
	_, err = svc.ToggleComplete(ctx, admin.ID, "", created.ID)
	require.ErrorIs(t, err, boom)

	got, err := reports.Get(ctx, created.ID)
	require.NoError(t, err)
	assert.False(t, got.Completed)
	assert.Equal(t, "pending", got.Status)

	var auditCount int
	err = tdb.Pool.QueryRow(ctx, `
		SELECT COUNT(*) FROM audit_logs
		WHERE action = $1 AND target_id = $2
	`, audit.ActionApproveReport, fmt.Sprint(created.ID)).Scan(&auditCount)
	require.NoError(t, err)
	assert.Zero(t, auditCount)

	var outboxCount int
	err = tdb.Pool.QueryRow(ctx, `
		SELECT COUNT(*) FROM report_notification_outbox WHERE report_id = $1
	`, created.ID).Scan(&outboxCount)
	require.NoError(t, err)
	assert.Zero(t, outboxCount)
}

func TestReportRepo(t *testing.T) {
	tdb := testutil.SetupPostgres(t)
	tdb.MigrateWithGoose(t)
	db := &database.DB{Pool: tdb.Pool, Builder: database.NewStatementBuilder()}
	repo := NewStore(db)
	ctx := context.Background()

	user, err := userimpl.NewStore(db).CreateWithPassword(ctx, "rep@example.com", "Rep", "hash")
	require.NoError(t, err)

	mk := func(month, worker string, metrics map[string]report.MetricWeeks) report.CreateReportRequest {
		return report.CreateReportRequest{Month: month, Worker: worker, ChurchName: "Bethel", AreaAssignment: "North",
			Names: "Ana", NarrativeReport: "n", Challenges: "challenges", PrayerRequest: "prayer", Metrics: metrics}
	}
	create := func(req report.CreateReportRequest, year int) *report.Report {
		t.Helper()
		rep, err := repo.Create(ctx, user.ID, year, req)
		require.NoError(t, err)
		return rep
	}

	jan := create(mk("January", "Ana_100%", map[string]report.MetricWeeks{
		"tithes_offering": {Week1: num("120.00"), Week3: num("0.5")},
	}), 2026)
	assert.Equal(t, "pending", jan.Status)

	t.Run("numeric round-trips as text, absent weeks omitted", func(t *testing.T) {
		got, err := repo.Get(ctx, jan.ID)
		require.NoError(t, err)
		w := got.ToResponse().Metrics["tithes_offering"]
		require.NotNil(t, w.Week1)
		assert.Equal(t, "120.00", string(*w.Week1))
		assert.Equal(t, "0.50", string(*w.Week3))
		assert.Nil(t, w.Week2)
		assert.Equal(t, "n", *got.NarrativeReport)
		assert.Equal(t, "challenges", *got.Challenges)
		assert.Equal(t, "prayer", *got.PrayerRequest)
	})

	t.Run("duplicate month/year is database.ErrUniqueViolation", func(t *testing.T) {
		_, err := repo.Create(ctx, user.ID, 2026, mk("January", "", nil))
		require.ErrorIs(t, err, database.ErrUniqueViolation)
	})

	t.Run("detail list keeps text and metrics for current reports and exports", func(t *testing.T) {
		got, total, err := repo.ListWithDetails(ctx, report.ListReportsParams{CreatedBy: user.ID, Month: "January"}, pagination.Params{Page: 1, Limit: 10})
		require.NoError(t, err)
		assert.EqualValues(t, 1, total)
		require.Len(t, got, 1)
		assert.Equal(t, jan.ID, got[0].ID)
		assert.Equal(t, "n", *got[0].NarrativeReport)
		assert.Equal(t, "challenges", *got[0].Challenges)
		assert.Equal(t, "prayer", *got[0].PrayerRequest)
		assert.Equal(t, "120.00", string(*got[0].ToResponse().Metrics["tithes_offering"].Week1))
	})

	t.Run("list filters escape wildcards and return summaries", func(t *testing.T) {
		create(mk("February", "Bob", map[string]report.MetricWeeks{"outreach": {Week1: num("3")}}), 2026)
		pg := pagination.Params{Page: 1, Limit: 10}

		got, total, err := repo.List(ctx, report.ListReportsParams{Worker: "100%"}, pg)
		require.NoError(t, err)
		assert.EqualValues(t, 1, total)
		require.Len(t, got, 1)
		assert.Equal(t, jan.ID, got[0].ID)
		assert.Nil(t, got[0].Metrics)
		assert.Nil(t, got[0].Names)
		assert.Nil(t, got[0].NarrativeReport)
		assert.Nil(t, got[0].Challenges)
		assert.Nil(t, got[0].PrayerRequest)
		assert.Equal(t, "Bethel", *got[0].ChurchName)
		assert.Equal(t, "North", *got[0].AreaAssignment)
		assert.Equal(t, user.ID, got[0].CreatedBy)
		assert.Equal(t, "pending", got[0].Status)
		assert.Equal(t, jan.CreatedAt, got[0].CreatedAt)

		// "_" must not act as a single-char wildcard: "A_a" would match "Ana" if unescaped
		_, total, err = repo.List(ctx, report.ListReportsParams{Worker: "A_a"}, pg)
		require.NoError(t, err)
		assert.EqualValues(t, 0, total)

		got, total, err = repo.List(ctx, report.ListReportsParams{Month: "FEBRUARY", Year: 2026}, pg)
		require.NoError(t, err)
		assert.EqualValues(t, 1, total)
		assert.Nil(t, got[0].Metrics)

		got, total, err = repo.List(ctx, report.ListReportsParams{}, pagination.Params{Page: 2, Limit: 1})
		require.NoError(t, err)
		assert.EqualValues(t, 2, total)
		assert.Len(t, got, 1)

		// Past the last page there is no row to carry COUNT(*) OVER(), but the total still holds.
		got, total, err = repo.List(ctx, report.ListReportsParams{}, pagination.Params{Page: 5, Limit: 1})
		require.NoError(t, err)
		assert.EqualValues(t, 2, total)
		assert.Empty(t, got)
	})

	t.Run("list can be limited to one owner", func(t *testing.T) {
		other, err := userimpl.NewStore(db).CreateWithPassword(ctx, "other@example.com", "Other", "hash")
		require.NoError(t, err)
		_, err = repo.Create(ctx, other.ID, 2026, mk("March", "", nil))
		require.NoError(t, err)

		got, total, err := repo.List(ctx, report.ListReportsParams{CreatedBy: other.ID}, pagination.Params{Page: 1, Limit: 10})
		require.NoError(t, err)
		assert.EqualValues(t, 1, total)
		assert.Equal(t, other.ID, got[0].CreatedBy)
	})

	t.Run("update replaces metrics and keeps completed fields", func(t *testing.T) {
		upd := report.UpdateReportRequest(mk("January", "Ana", map[string]report.MetricWeeks{"outreach": {Week2: num("7.25")}}))
		got, err := repo.Update(ctx, jan.ID, 2026, upd)
		require.NoError(t, err)
		assert.NotContains(t, got.Metrics, "tithes_offering")
		assert.Equal(t, "7.25", string(*got.ToResponse().Metrics["outreach"].Week2))

		_, err = repo.Update(ctx, 99999, 2026, upd)
		require.ErrorIs(t, err, database.ErrNotFound)

		// moving onto another active (month, year) collides
		upd.Month = "February"
		_, err = repo.Update(ctx, jan.ID, 2026, upd)
		require.ErrorIs(t, err, database.ErrUniqueViolation)
	})

	t.Run("revision update returns report to pending and records user notes", func(t *testing.T) {
		_, err := repo.RequestRevision(ctx, jan.ID, user.ID, "Please clarify the totals")
		require.NoError(t, err)

		updatedReq := report.UpdateReportRequest(mk("January", "Ana", nil))
		updatedReq.UserNotes = "I corrected the weekly totals."
		updated, err := repo.Update(ctx, jan.ID, 2026, updatedReq)
		require.NoError(t, err)
		assert.Equal(t, "pending", updated.Status)
		assert.NotNil(t, updated.LastRevisedAt)
		require.NotNil(t, updated.UserNotes)
		assert.Equal(t, "I corrected the weekly totals.", *updated.UserNotes)

		got, total, err := repo.List(ctx, report.ListReportsParams{Revised: true}, pagination.Params{Page: 1, Limit: 10})
		require.NoError(t, err)
		assert.EqualValues(t, 1, total)
		assert.Equal(t, jan.ID, got[0].ID)
	})

	t.Run("toggle approves then unapproves", func(t *testing.T) {
		on, err := repo.ToggleComplete(ctx, jan.ID, user.ID)
		require.NoError(t, err)
		require.True(t, on.Completed)
		require.NotNil(t, on.CompletedAt)
		assert.Equal(t, &user.ID, on.CompletedBy)

		off, err := repo.ToggleComplete(ctx, jan.ID, user.ID)
		require.NoError(t, err)
		assert.False(t, off.Completed)
		assert.Nil(t, off.CompletedAt)
		assert.Nil(t, off.CompletedBy)
	})

	t.Run("soft-deleted report is invisible everywhere and its period is reusable", func(t *testing.T) {
		require.NoError(t, repo.SoftDelete(ctx, jan.ID))

		got, err := repo.Get(ctx, jan.ID)
		require.NoError(t, err)
		assert.Nil(t, got)

		_, total, err := repo.List(ctx, report.ListReportsParams{Month: "January"}, pagination.Params{Page: 1, Limit: 10})
		require.NoError(t, err)
		assert.EqualValues(t, 0, total)

		require.ErrorIs(t, repo.SoftDelete(ctx, jan.ID), database.ErrNotFound)
		_, err = repo.ToggleComplete(ctx, jan.ID, user.ID)
		require.ErrorIs(t, err, database.ErrNotFound)
		_, err = repo.Update(ctx, jan.ID, 2026, report.UpdateReportRequest(mk("January", "", nil)))
		require.ErrorIs(t, err, database.ErrNotFound)

		_, err = repo.Create(ctx, user.ID, 2026, mk("January", "", nil))
		require.NoError(t, err)
	})
}

// The partial unique index (created_by, month, year) WHERE deleted_at IS NULL is
// the guarantee: of N simultaneous creates for one period, exactly one wins.
func TestReportRepo_ConcurrentCreateSamePeriod(t *testing.T) {
	tdb := testutil.SetupPostgres(t)
	tdb.MigrateWithGoose(t)
	db := &database.DB{Pool: tdb.Pool, Builder: database.NewStatementBuilder()}
	repo := NewStore(db)
	ctx := context.Background()

	user, err := userimpl.NewStore(db).CreateWithPassword(ctx, "race@example.com", "Race", "hash")
	require.NoError(t, err)

	const n = 8
	req := report.CreateReportRequest{Month: "March", Metrics: map[string]report.MetricWeeks{"outreach": {Week1: num("1")}}}
	start := make(chan struct{})
	errs := make(chan error, n)
	for i := 0; i < n; i++ {
		go func() {
			<-start
			_, err := repo.Create(ctx, user.ID, 2026, req)
			errs <- err
		}()
	}
	close(start)

	var ok, dup int
	for i := 0; i < n; i++ {
		switch err := <-errs; {
		case err == nil:
			ok++
		case errors.Is(err, database.ErrUniqueViolation):
			dup++
		default:
			t.Errorf("unexpected error: %v", err)
		}
	}
	assert.Equal(t, 1, ok, "exactly one create wins")
	assert.Equal(t, n-1, dup, "the rest are database.ErrUniqueViolation")

	var reports, metrics int
	require.NoError(t, db.Pool.QueryRow(ctx, `SELECT count(*) FROM reports WHERE created_by = $1`, user.ID).Scan(&reports))
	require.NoError(t, db.Pool.QueryRow(ctx, `SELECT count(*) FROM report_metrics`).Scan(&metrics))
	assert.Equal(t, 1, reports)
	assert.Equal(t, 1, metrics, "losers insert no metrics")
}

// Owner state is current metadata, not a snapshot at report creation.
func TestReportRepo_OwnerState(t *testing.T) {
	tdb := testutil.SetupPostgres(t)
	tdb.MigrateWithGoose(t)
	db := &database.DB{Pool: tdb.Pool, Builder: database.NewStatementBuilder()}
	repo := NewStore(db)
	users := userimpl.NewStore(db)
	ctx := context.Background()
	owner, err := users.CreateWithPassword(ctx, "owner-state@example.com", "Owner", "hash")
	require.NoError(t, err)
	actor, err := users.CreateWithPassword(ctx, "actor-state@example.com", "Actor", "hash")
	require.NoError(t, err)
	req := report.CreateReportRequest{Month: "January", Worker: "Historical worker", Metrics: map[string]report.MetricWeeks{"outreach": {Week1: num("3")}}}
	created, err := repo.Create(ctx, owner.ID, 2026, req)
	require.NoError(t, err)
	assert.Equal(t, "active", created.OwnerStatus)
	assert.Nil(t, created.OwnerDeletedAt)
	_, err = repo.Create(ctx, owner.ID, 2026, report.CreateReportRequest{Month: "February"})
	require.NoError(t, err)

	for _, tt := range []struct {
		name, status string
		deleted      bool
	}{
		{"active", "active", false},
		{"inactive", "inactive", false},
		{"disciplinary", "disciplinary", false},
		{"deleted", "active", true},
		{"deceased and deleted", "deceased", true},
		{"restored deceased", "deceased", false},
		{"reactivated", "active", false},
	} {
		t.Run(tt.name, func(t *testing.T) {
			_, err := tdb.Pool.Exec(ctx, "UPDATE users SET status=$1 WHERE id=$2", tt.status, owner.ID)
			require.NoError(t, err)
			if tt.deleted {
				_, err = users.SoftDelete(ctx, owner.ID, actor.ID, "private reason")
			} else {
				_, err = users.Restore(ctx, owner.ID)
			}
			require.NoError(t, err)
			got, err := repo.Get(ctx, created.ID)
			require.NoError(t, err)
			require.NotNil(t, got)
			assert.Equal(t, tt.status, got.OwnerStatus)
			assert.Equal(t, tt.deleted, got.OwnerDeletedAt != nil)
			assert.Equal(t, "Historical worker", *got.Worker)
			assert.Contains(t, got.Metrics, "outreach")
			for page := 1; page <= 2; page++ {
				rows, total, err := repo.List(ctx, report.ListReportsParams{CreatedBy: owner.ID}, pagination.Params{Page: page, Limit: 1})
				require.NoError(t, err)
				assert.EqualValues(t, 2, total)
				require.Len(t, rows, 1)
				assert.Equal(t, tt.status, rows[0].OwnerStatus)
				assert.Equal(t, tt.deleted, rows[0].OwnerDeletedAt != nil)
			}
			updated, err := repo.Update(ctx, created.ID, 2026, report.UpdateReportRequest(req))
			require.NoError(t, err)
			assert.Equal(t, tt.status, updated.OwnerStatus)
			assert.Equal(t, tt.deleted, updated.OwnerDeletedAt != nil)
			// Every full response, including a write, carries current owner state.
			newReport, err := repo.Create(ctx, owner.ID, 2027, req)
			require.NoError(t, err)
			assert.Equal(t, tt.status, newReport.OwnerStatus)
			assert.Equal(t, tt.deleted, newReport.OwnerDeletedAt != nil)
			require.NoError(t, repo.SoftDelete(ctx, newReport.ID))
		})
	}
	require.NoError(t, repo.SoftDelete(ctx, created.ID))
	got, err := repo.Get(ctx, created.ID)
	require.NoError(t, err)
	assert.Nil(t, got)
	rows, total, err := repo.List(ctx, report.ListReportsParams{CreatedBy: owner.ID}, pagination.Params{Page: 1, Limit: 10})
	require.NoError(t, err)
	assert.EqualValues(t, 1, total)
	require.Len(t, rows, 1)
}

func TestReportRepo_Dashboard(t *testing.T) {
	tdb := testutil.SetupPostgres(t)
	tdb.MigrateWithGoose(t)
	db := &database.DB{Pool: tdb.Pool, Builder: database.NewStatementBuilder()}
	repo := NewStore(db)
	users := userimpl.NewStore(db)
	ctx := context.Background()

	account := func(email, name, church, area string) int64 {
		t.Helper()
		u, err := users.CreateWithPassword(ctx, email, name, "hash")
		require.NoError(t, err)
		_, err = users.UpdateProfile(ctx, u.ID, user.UserProfile{Name: name, AreaAssignment: area, ChurchName: church})
		require.NoError(t, err)
		return u.ID
	}
	file := func(owner int64, month string, year int, approve bool) *report.Report {
		t.Helper()
		rep, err := repo.Create(ctx, owner, year, report.CreateReportRequest{Month: month})
		require.NoError(t, err)
		if approve {
			rep, err = repo.ToggleComplete(ctx, rep.ID, owner)
			require.NoError(t, err)
		}
		return rep
	}

	approved := account("zion@example.com", "Ana", "zion", "Leyte")
	pending := account("bethel@example.com", "Ben", "Bethel", "Cebu")
	notFiled := account("abba@example.com", "Cora", "Abba", "")
	gone := account("gone@example.com", "Dan", "Gone Church", "x")
	deceased := account("deceased@example.com", "Eva", "Calvary", "y")
	account("blank@example.com", "Fe", "  ", "z")
	admin := account("admin-dash@example.com", "Gil", "Admin Church", "w")
	deletedReport := account("redo@example.com", "Hal", "Redo", "v")
	editor := account("editor-dash@example.com", "Ivy", "Berea", "u") // not the user role (D-A24)

	file(approved, "May", 2026, true)
	file(pending, "May", 2026, false)
	file(notFiled, "April", 2026, true) // another month
	file(notFiled, "May", 2025, true)   // another year
	file(deceased, "May", 2026, true)
	file(admin, "May", 2026, true)
	file(editor, "May", 2026, true)
	require.NoError(t, repo.SoftDelete(ctx, file(deletedReport, "May", 2026, true).ID))

	_, err := tdb.Pool.Exec(ctx, "UPDATE users SET status='deceased' WHERE id=$1", deceased)
	require.NoError(t, err)
	// Roles are seed data (seed.go), not migrations, so the admin role is added here.
	_, err = tdb.Pool.Exec(ctx, "INSERT INTO roles (name) VALUES ('admin') ON CONFLICT (name) DO NOTHING")
	require.NoError(t, err)
	_, err = tdb.Pool.Exec(ctx, "UPDATE users SET role_id=(SELECT id FROM roles WHERE name='admin') WHERE id=$1", admin)
	require.NoError(t, err)
	_, err = tdb.Pool.Exec(ctx, "INSERT INTO roles (name) VALUES ('editor') ON CONFLICT (name) DO NOTHING")
	require.NoError(t, err)
	_, err = tdb.Pool.Exec(ctx, "UPDATE users SET role_id=(SELECT id FROM roles WHERE name='editor') WHERE id=$1", editor)
	require.NoError(t, err)
	_, err = users.SoftDelete(ctx, gone, approved, "")
	require.NoError(t, err)

	rows, err := repo.Dashboard(ctx, "May", 2026)
	require.NoError(t, err)
	assert.Equal(t, []report.DashboardRow{
		{Church: "Abba", Status: report.StatusNoReport}, // other months only
		{Church: "Bethel", Month: "May", Year: 2026, Status: report.StatusPending},
		{Church: "Calvary", Month: "May", Year: 2026, Status: report.StatusApproved},
		{Church: "Redo", Status: report.StatusNoReport}, // its report is soft-deleted
		{Church: "zion", Month: "May", Year: 2026, Status: report.StatusApproved},
	}, rows)

	t.Run("Completed filters List", func(t *testing.T) {
		yes, no := true, false
		approvedReps, total, err := repo.List(ctx, report.ListReportsParams{Month: "May", Year: 2026, Completed: &yes}, pagination.Params{Page: 1, Limit: 100})
		require.NoError(t, err)
		assert.EqualValues(t, 4, total) // zion, Calvary, the admin's, the editor's
		for _, r := range approvedReps {
			assert.True(t, r.Completed)
		}
		_, total, err = repo.List(ctx, report.ListReportsParams{Month: "May", Year: 2026, Completed: &no}, pagination.Params{Page: 1, Limit: 100})
		require.NoError(t, err)
		assert.EqualValues(t, 1, total) // Bethel
	})
}
