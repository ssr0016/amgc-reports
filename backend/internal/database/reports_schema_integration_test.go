//go:build integration

package database_test

import (
	"context"
	"database/sql"
	"errors"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/jackc/pgx/v5/stdlib"
	"github.com/pressly/goose/v3"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ssr0016/pastoral-reports/backend/internal/database"
	"github.com/ssr0016/pastoral-reports/backend/internal/testutil"
)

const (
	checkViolation      = "23514"
	uniqueViolation     = "23505"
	foreignKeyViolation = "23503"
	stringTooLong       = "22001"
	numericOutOfRange   = "22003"

	// Last migration that exists before the Reports schema.
	lastPreReportsVersion = 20251012000002
)

// The 21 metric keys of D-6 (snake_case).
var metricKeys = []string{
	"worship_service", "sunday_school", "prayer_meeting", "bible_studies",
	"mens_fellowship", "womens_fellowship", "youth_fellowship", "children_fellowship",
	"tithes_offering", "home_visited", "bible_study_group_led", "sermon_preached",
	"person_newly_contacted", "person_followed_up", "person_evangelized", "outreach",
	"training", "leadership", "baptism", "other", "family_day",
}

var auditActions = []string{
	"CREATE_REPORT", "UPDATE_REPORT", "DELETE_REPORT",
	"APPROVE_REPORT", "UNAPPROVE_REPORT",
	"DOWNLOAD_SINGLE_EXCEL", "DOWNLOAD_BULK_EXCEL",
}

// reportFields are the client-settable columns of a report row.
type reportFields struct {
	Month       string
	Year        int
	Worker      *string
	Names       *string
	Narrative   *string
	Challenges  *string
	Prayer      *string
	Completed   bool
	CompletedAt *time.Time
}

func validReport() reportFields {
	return reportFields{Month: "January", Year: 2026}
}

const insertReportSQL = `
	INSERT INTO reports (created_by, month, year, worker, names, narrative_report,
	                     challenges, prayer_request, completed, completed_at)
	VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)
	RETURNING id`

func migratedDB(t *testing.T) *testutil.TestDB {
	t.Helper()
	tdb := testutil.SetupPostgres(t)
	tdb.MigrateWithGoose(t)
	return tdb
}

func insertUser(t *testing.T, pool *pgxpool.Pool, email string) int64 {
	t.Helper()
	var id int64
	require.NoError(t, pool.QueryRow(context.Background(),
		`INSERT INTO users (email, name, password_hash, role_id)
		 VALUES ($1, 'T', 'x', (SELECT id FROM roles WHERE name = 'user')) RETURNING id`,
		email).Scan(&id), "insert user %s", email)
	return id
}

func insertReportRow(pool *pgxpool.Pool, userID int64, f reportFields) (int64, error) {
	var id int64
	err := pool.QueryRow(context.Background(), insertReportSQL,
		userID, f.Month, f.Year, f.Worker, f.Names, f.Narrative, f.Challenges, f.Prayer,
		f.Completed, f.CompletedAt).Scan(&id)
	return id, err
}

func insertReport(t *testing.T, pool *pgxpool.Pool, userID int64, month string, year int) int64 {
	t.Helper()
	f := validReport()
	f.Month, f.Year = month, year
	id, err := insertReportRow(pool, userID, f)
	require.NoError(t, err, "insert report %s %d", month, year)
	return id
}

func insertMetric(pool *pgxpool.Pool, reportID int64, metric string, week int, value string) error {
	_, err := pool.Exec(context.Background(),
		`INSERT INTO report_metrics (report_id, metric, week_no, value) VALUES ($1, $2, $3, $4::numeric)`,
		reportID, metric, week, value)
	return err
}

func insertAudit(pool *pgxpool.Pool, action string, performedBy *int64, ip string) error {
	_, err := pool.Exec(context.Background(),
		`INSERT INTO audit_logs (performed_by, action, target_type, target_id, ip_address)
		 VALUES ($1, $2, 'Report', '1', $3)`,
		performedBy, action, ip)
	return err
}

func requirePgError(t *testing.T, err error, code, constraint string) {
	t.Helper()
	var pgErr *pgconn.PgError
	require.True(t, errors.As(err, &pgErr), "want a Postgres error, got %v", err)
	assert.Equal(t, code, pgErr.Code, pgErr.Message)
	if constraint != "" {
		assert.Equal(t, constraint, pgErr.ConstraintName)
	}
}

func relationExists(t *testing.T, pool *pgxpool.Pool, name string) bool {
	t.Helper()
	var exists bool
	require.NoError(t, pool.QueryRow(context.Background(),
		`SELECT to_regclass($1) IS NOT NULL`, "public."+name).Scan(&exists))
	return exists
}

func chars(n int) *string {
	s := strings.Repeat("x", n)
	return &s
}

// exerciseReportsDML runs the statements the server issues against the Reports
// tables, as the given pool, and fails the test on the first one refused.
func exerciseReportsDML(t *testing.T, pool *pgxpool.Pool, userID int64) {
	t.Helper()
	ctx := context.Background()

	reportID := insertReport(t, pool, userID, "March", 2030)
	require.NoError(t, insertMetric(pool, reportID, "worship_service", 1, "12.50"), "insert report_metrics")
	_, err := pool.Exec(ctx, `UPDATE reports SET updated_at = NOW() WHERE id = $1`, reportID)
	require.NoError(t, err, "update reports")
	_, err = pool.Exec(ctx, `UPDATE report_metrics SET value = 13 WHERE report_id = $1`, reportID)
	require.NoError(t, err, "update report_metrics")
	var n int
	require.NoError(t, pool.QueryRow(ctx,
		`SELECT COUNT(*) FROM report_metrics WHERE report_id = $1`, reportID).Scan(&n), "select report_metrics")
	assert.Equal(t, 1, n)
	_, err = pool.Exec(ctx, `DELETE FROM report_metrics WHERE report_id = $1`, reportID)
	require.NoError(t, err, "delete report_metrics")
	_, err = pool.Exec(ctx, `DELETE FROM reports WHERE id = $1`, reportID)
	require.NoError(t, err, "delete reports")
}

func migrateUpTo(t *testing.T, tdb *testutil.TestDB, version int64) {
	t.Helper()
	db := sql.OpenDB(stdlib.GetPoolConnector(tdb.Pool))
	t.Cleanup(func() { _ = db.Close() })
	goose.SetBaseFS(os.DirFS("migrations"))
	require.NoError(t, goose.SetDialect("postgres"))
	require.NoError(t, goose.UpTo(db, ".", version))
}

func TestReportsSchema_ObjectsExist(t *testing.T) {
	tdb := migratedDB(t)

	for _, table := range []string{"reports", "report_metrics", "audit_logs"} {
		assert.True(t, relationExists(t, tdb.Pool, table), "table %s was not created", table)
	}

	rows, err := tdb.Pool.Query(context.Background(),
		`SELECT indexname FROM pg_indexes WHERE schemaname = 'public'`)
	require.NoError(t, err)
	defer rows.Close()
	indexes := map[string]bool{}
	for rows.Next() {
		var name string
		require.NoError(t, rows.Scan(&name))
		indexes[name] = true
	}
	require.NoError(t, rows.Err())

	for _, want := range []string{
		"idx_reports_unique_per_user", "idx_reports_year_created_at", "idx_reports_year_month",
		"idx_audit_logs_created_at", "idx_audit_logs_target", "idx_audit_logs_action",
	} {
		assert.True(t, indexes[want], "index %s is missing", want)
	}
	// D-11: filters are ILIKE '%x%', which a btree cannot serve.
	for _, unwanted := range []string{
		"idx_reports_church_name", "idx_reports_area_assignment", "idx_reports_worker",
	} {
		assert.False(t, indexes[unwanted], "index %s must not exist (D-11)", unwanted)
	}
}

func TestReports_CheckConstraints(t *testing.T) {
	tdb := migratedDB(t)
	userID := insertUser(t, tdb.Pool, "check@example.com")
	now := time.Now()

	tests := []struct {
		name       string
		mutate     func(*reportFields)
		constraint string
	}{
		{"month not an English name", func(f *reportFields) { f.Month = "Sept" }, "chk_reports_month"},
		{"year below range", func(f *reportFields) { f.Year = 1999 }, "chk_reports_year"},
		{"year above range", func(f *reportFields) { f.Year = 2101 }, "chk_reports_year"},
		{"names too long", func(f *reportFields) { f.Names = chars(5001) }, "chk_reports_text_len"},
		{"narrative too long", func(f *reportFields) { f.Narrative = chars(10001) }, "chk_reports_text_len"},
		{"challenges too long", func(f *reportFields) { f.Challenges = chars(10001) }, "chk_reports_text_len"},
		{"prayer request too long", func(f *reportFields) { f.Prayer = chars(10001) }, "chk_reports_text_len"},
		{"completed without completed_at", func(f *reportFields) { f.Completed = true }, "chk_reports_completed"},
		{"completed_at without completed", func(f *reportFields) { f.CompletedAt = &now }, "chk_reports_completed"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			f := validReport()
			tt.mutate(&f)

			_, err := insertReportRow(tdb.Pool, userID, f)

			requirePgError(t, err, checkViolation, tt.constraint)
		})
	}
}

func TestReports_Boundaries_Accepted(t *testing.T) {
	tdb := migratedDB(t)
	userID := insertUser(t, tdb.Pool, "bounds@example.com")

	multibyte := strings.Repeat("é", 5000) // 5000 characters, 10000 bytes: the limit counts characters
	for _, f := range []reportFields{
		{Month: "January", Year: 2000},
		{Month: "January", Year: 2100, Names: &multibyte, Worker: chars(150),
			Narrative: chars(10000), Challenges: chars(10000), Prayer: chars(10000)},
	} {
		_, err := insertReportRow(tdb.Pool, userID, f)
		require.NoError(t, err, "year %d", f.Year)
	}

	f := validReport()
	f.Year = 2050
	f.Worker = chars(151)
	_, err := insertReportRow(tdb.Pool, userID, f)
	requirePgError(t, err, stringTooLong, "")
}

func TestReports_UniquePerUserMonthYear(t *testing.T) {
	tdb := migratedDB(t)
	alice := insertUser(t, tdb.Pool, "alice@example.com")
	bob := insertUser(t, tdb.Pool, "bob@example.com")

	first := insertReport(t, tdb.Pool, alice, "January", 2026)

	_, err := insertReportRow(tdb.Pool, alice, validReport())
	requirePgError(t, err, uniqueViolation, "idx_reports_unique_per_user")

	insertReport(t, tdb.Pool, bob, "January", 2026)    // other user
	insertReport(t, tdb.Pool, alice, "February", 2026) // other month
	insertReport(t, tdb.Pool, alice, "January", 2027)  // other year

	// A soft-deleted report is outside the partial index.
	_, err = tdb.Pool.Exec(context.Background(), `UPDATE reports SET deleted_at = NOW() WHERE id = $1`, first)
	require.NoError(t, err)
	insertReport(t, tdb.Pool, alice, "January", 2026)

	_, err = insertReportRow(tdb.Pool, alice, validReport())
	requirePgError(t, err, uniqueViolation, "idx_reports_unique_per_user")
}

func TestReports_CreatedByRestrict(t *testing.T) {
	tdb := migratedDB(t)
	ctx := context.Background()
	author := insertUser(t, tdb.Pool, "author@example.com")
	reportID := insertReport(t, tdb.Pool, author, "January", 2026)

	_, err := tdb.Pool.Exec(ctx, `DELETE FROM users WHERE id = $1`, author)
	requirePgError(t, err, foreignKeyViolation, "")

	// A soft-deleted report still blocks: the row is still there.
	_, err = tdb.Pool.Exec(ctx, `UPDATE reports SET deleted_at = NOW() WHERE id = $1`, reportID)
	require.NoError(t, err)
	_, err = tdb.Pool.Exec(ctx, `DELETE FROM users WHERE id = $1`, author)
	requirePgError(t, err, foreignKeyViolation, "")
}

func TestReports_CompletedBySetNull(t *testing.T) {
	tdb := migratedDB(t)
	ctx := context.Background()
	author := insertUser(t, tdb.Pool, "writer@example.com")
	approver := insertUser(t, tdb.Pool, "approver@example.com")

	var reportID int64
	require.NoError(t, tdb.Pool.QueryRow(ctx,
		`INSERT INTO reports (created_by, month, year, completed, completed_at, completed_by)
		 VALUES ($1, 'January', 2026, TRUE, NOW(), $2) RETURNING id`, author, approver).Scan(&reportID))

	_, err := tdb.Pool.Exec(ctx, `DELETE FROM users WHERE id = $1`, approver)
	require.NoError(t, err)

	var completedBy *int64
	var completed bool
	require.NoError(t, tdb.Pool.QueryRow(ctx,
		`SELECT completed_by, completed FROM reports WHERE id = $1`, reportID).Scan(&completedBy, &completed))
	assert.Nil(t, completedBy)
	assert.True(t, completed, "the report must survive the approver's deletion")
}

func TestReportMetrics_Constraints(t *testing.T) {
	tdb := migratedDB(t)
	userID := insertUser(t, tdb.Pool, "metrics@example.com")
	reportID := insertReport(t, tdb.Pool, userID, "January", 2026)

	t.Run("all 21 snake_case keys and weeks 1..5 are accepted", func(t *testing.T) {
		for _, key := range metricKeys {
			require.NoError(t, insertMetric(tdb.Pool, reportID, key, 1, "0"), "key %s", key)
		}
		for week := 2; week <= 5; week++ {
			require.NoError(t, insertMetric(tdb.Pool, reportID, "worship_service", week, "120.50"), "week %d", week)
		}
	})

	tests := []struct {
		name       string
		metric     string
		week       int
		value      string
		code       string
		constraint string
	}{
		{"camelCase key (D-6)", "worshipService", 1, "1", checkViolation, "chk_report_metrics_metric"},
		{"unknown key", "no_such_metric", 1, "1", checkViolation, "chk_report_metrics_metric"},
		{"week 0", "outreach", 0, "1", checkViolation, "chk_report_metrics_week"},
		{"week 6", "outreach", 6, "1", checkViolation, "chk_report_metrics_week"},
		{"negative value", "outreach", 2, "-0.01", checkViolation, "chk_report_metrics_value"},
		{"duplicate metric and week", "worship_service", 1, "5", uniqueViolation, "report_metrics_pkey"},
		{"value above NUMERIC(12,2)", "outreach", 3, "10000000000.00", numericOutOfRange, ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := insertMetric(tdb.Pool, reportID, tt.metric, tt.week, tt.value)

			requirePgError(t, err, tt.code, tt.constraint)
		})
	}

	t.Run("largest value is accepted", func(t *testing.T) {
		require.NoError(t, insertMetric(tdb.Pool, reportID, "tithes_offering", 2, "9999999999.99"))
	})

	t.Run("sparse: a report without metrics is valid", func(t *testing.T) {
		other := insertReport(t, tdb.Pool, userID, "February", 2026)
		var n int
		require.NoError(t, tdb.Pool.QueryRow(context.Background(),
			`SELECT COUNT(*) FROM report_metrics WHERE report_id = $1`, other).Scan(&n))
		assert.Zero(t, n)
	})
}

func TestReportMetrics_CascadeOnHardDelete(t *testing.T) {
	tdb := migratedDB(t)
	ctx := context.Background()
	userID := insertUser(t, tdb.Pool, "cascade@example.com")
	soft := insertReport(t, tdb.Pool, userID, "January", 2026)
	hard := insertReport(t, tdb.Pool, userID, "February", 2026)
	require.NoError(t, insertMetric(tdb.Pool, soft, "outreach", 1, "1"))
	require.NoError(t, insertMetric(tdb.Pool, hard, "outreach", 1, "1"))

	_, err := tdb.Pool.Exec(ctx, `UPDATE reports SET deleted_at = NOW() WHERE id = $1`, soft)
	require.NoError(t, err)
	_, err = tdb.Pool.Exec(ctx, `DELETE FROM reports WHERE id = $1`, hard)
	require.NoError(t, err)

	count := func(id int64) (n int) {
		require.NoError(t, tdb.Pool.QueryRow(ctx,
			`SELECT COUNT(*) FROM report_metrics WHERE report_id = $1`, id).Scan(&n))
		return n
	}
	assert.Equal(t, 1, count(soft), "a soft delete must leave the metrics")
	assert.Zero(t, count(hard), "a hard delete must remove the metrics")
}

func TestAuditLogs_Constraints(t *testing.T) {
	tdb := migratedDB(t)
	ctx := context.Background()

	for _, action := range auditActions {
		require.NoError(t, insertAudit(tdb.Pool, action, nil, "203.0.113.7"), "action %s", action)
	}

	requirePgError(t, insertAudit(tdb.Pool, "DROP_TABLE", nil, "203.0.113.7"),
		checkViolation, "chk_audit_logs_action")

	require.NoError(t, insertAudit(tdb.Pool, "CREATE_REPORT", nil, strings.Repeat("a", 64)))
	requirePgError(t, insertAudit(tdb.Pool, "CREATE_REPORT", nil, strings.Repeat("a", 65)),
		stringTooLong, "")

	var details string
	require.NoError(t, tdb.Pool.QueryRow(ctx,
		`SELECT details::text FROM audit_logs ORDER BY id LIMIT 1`).Scan(&details))
	assert.Equal(t, "{}", details, "details must default to an empty object")
}

func TestAuditLogs_AppUserIsAppendOnly(t *testing.T) {
	tdb := migratedDB(t)
	ctx := context.Background()
	app := tdb.AppUserPool(t)

	require.NoError(t, insertAudit(app, "CREATE_REPORT", nil, "203.0.113.7"), "app_user must be able to insert")
	var n int
	require.NoError(t, app.QueryRow(ctx, `SELECT COUNT(*) FROM audit_logs`).Scan(&n), "app_user must be able to select")
	assert.Equal(t, 1, n)

	_, err := app.Exec(ctx, `UPDATE audit_logs SET action = 'DELETE_REPORT'`)
	requirePgError(t, err, insufficientPrivilege, "")
	_, err = app.Exec(ctx, `DELETE FROM audit_logs`)
	requirePgError(t, err, insufficientPrivilege, "")

	// The log keeps its rows when the actor is deleted; the foreign key action
	// runs with the table owner's rights, not app_user's.
	actor := insertUser(t, app, "actor@example.com")
	require.NoError(t, insertAudit(app, "UPDATE_REPORT", &actor, "203.0.113.7"))
	_, err = app.Exec(ctx, `DELETE FROM users WHERE id = $1`, actor)
	require.NoError(t, err, "deleting an audited user must not need UPDATE on audit_logs")

	var performedBy *int64
	require.NoError(t, tdb.Pool.QueryRow(ctx,
		`SELECT performed_by FROM audit_logs WHERE action = 'UPDATE_REPORT'`).Scan(&performedBy))
	assert.Nil(t, performedBy)
}

func TestReportsTables_AppUserDML(t *testing.T) {
	tdb := migratedDB(t)
	app := tdb.AppUserPool(t)
	userID := insertUser(t, app, "dml@example.com")

	exerciseReportsDML(t, app, userID)
}

// This is the production path: `api migrate` connects as migrate_user.
func TestReportsMigrations_AsMigrateUser(t *testing.T) {
	tdb := testutil.SetupPostgres(t)
	ctx := context.Background()
	migrateUpTo(t, tdb, lastPreReportsVersion)
	mig := tdb.RolePool(t, "migrate_user")

	require.NoError(t, database.RunMigrations(mig), "migrate_user could not apply the Reports migrations")

	for _, table := range []string{"reports", "report_metrics", "audit_logs"} {
		var owner string
		require.NoError(t, tdb.Pool.QueryRow(ctx,
			`SELECT tableowner FROM pg_tables WHERE schemaname = 'public' AND tablename = $1`, table).Scan(&owner),
			"table %s", table)
		assert.Equal(t, "migrate_user", owner, "table %s", table)
	}

	app := tdb.AppUserPool(t)
	exerciseReportsDML(t, app, insertUser(t, app, "prod@example.com"))

	_, err := app.Exec(ctx, `UPDATE audit_logs SET action = 'DELETE_REPORT'`)
	requirePgError(t, err, insufficientPrivilege, "")
	_, err = app.Exec(ctx, `DELETE FROM audit_logs`)
	requirePgError(t, err, insufficientPrivilege, "")
}

func TestReportsMigrations_DownThenUp(t *testing.T) {
	tdb := migratedDB(t)
	tables := []string{"reports", "report_metrics", "audit_logs"}
	for _, table := range tables {
		require.True(t, relationExists(t, tdb.Pool, table), "table %s was not created", table)
	}

	db := sql.OpenDB(stdlib.GetPoolConnector(tdb.Pool))
	defer func() { _ = db.Close() }()
	goose.SetBaseFS(os.DirFS("migrations"))
	require.NoError(t, goose.SetDialect("postgres"))
	// Newest first: audit_logs, report_metrics, reports.
	for _, name := range []string{"audit_logs", "report_metrics", "reports"} {
		require.NoError(t, goose.Down(db, "."), "down %s migration failed", name)
	}
	for _, table := range tables {
		assert.False(t, relationExists(t, tdb.Pool, table), "down left table %s behind", table)
	}

	tdb.MigrateWithGoose(t)
	for _, table := range tables {
		assert.True(t, relationExists(t, tdb.Pool, table), "up again did not recreate %s", table)
	}
}
