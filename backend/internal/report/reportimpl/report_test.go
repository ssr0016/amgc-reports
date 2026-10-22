package reportimpl

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/xuri/excelize/v2"

	"github.com/ssr0016/pastoral-reports/backend/internal/accesscontrol"
	"github.com/ssr0016/pastoral-reports/backend/internal/audit"
	"github.com/ssr0016/pastoral-reports/backend/internal/audit/auditimpl"
	"github.com/ssr0016/pastoral-reports/backend/internal/database"
	"github.com/ssr0016/pastoral-reports/backend/internal/report"
	"github.com/ssr0016/pastoral-reports/backend/internal/user"
	"github.com/ssr0016/pastoral-reports/backend/internal/user/userimpl"
	"github.com/ssr0016/pastoral-reports/backend/pkg/pagination"
)

// fakeTx runs fn directly. The in-memory mocks have no transaction to roll back.
type fakeTx struct{}

func (fakeTx) InTransaction(ctx context.Context, fn func(context.Context) error) error {
	return fn(ctx)
}

// stubRoles serves permissions by role ID; other methods panic if called.
type stubRoles struct {
	accesscontrol.RoleStore
	perms map[int64][]string
}

func (s stubRoles) GetPermissions(_ context.Context, roleID int64) ([]accesscontrol.Permission, error) {
	var out []accesscontrol.Permission
	for _, n := range s.perms[roleID] {
		out = append(out, accesscontrol.Permission{Name: n})
	}
	return out, nil
}

const (
	alice = int64(1) // plain user: own reports only
	bob   = int64(2) // editor: _any + approve
	carol = int64(3) // reader: read_any only, as the seeded user role
	dave  = int64(4) // read_any and delete_any, no write_any
	roleU = int64(10)
	roleE = int64(20)
	roleR = int64(30)
	roleD = int64(40)
)

type fixture struct {
	svc     *service
	reports *MockStore
	audit   *auditimpl.MockStore
}

func newFixture() fixture {
	users := userimpl.NewMockStore()
	users.GetWithRoleFunc = func(_ context.Context, id int64) (*user.User, error) {
		switch id {
		case alice:
			return &user.User{ID: alice, Name: "Alice", RoleID: roleU, Role: &accesscontrol.Role{Name: "user"}}, nil
		case bob:
			return &user.User{ID: bob, Name: "Bob", RoleID: roleE, Role: &accesscontrol.Role{Name: "editor"}}, nil
		case carol:
			return &user.User{ID: carol, Name: "Carol", RoleID: roleR, Role: &accesscontrol.Role{Name: "user"}}, nil
		case dave:
			return &user.User{ID: dave, Name: "Dave", RoleID: roleD, Role: &accesscontrol.Role{Name: "custom"}}, nil
		}
		return nil, nil
	}
	roles := stubRoles{perms: map[int64][]string{
		roleU: {"reports:read", "reports:write", "reports:delete"},
		roleE: {"reports:read", "reports:write", "reports:delete", "reports:read_any", "reports:write_any", "reports:delete_any"},
		roleR: {"reports:read", "reports:write", "reports:delete", "reports:read_any"},
		roleD: {"reports:read", "reports:read_any", "reports:delete_any"},
	}}
	f := fixture{reports: NewMockStore(), audit: auditimpl.NewMockStore()}
	f.svc = newService(f.reports, f.audit, fakeTx{}, users, roles)
	f.svc.now = func() time.Time { return time.Date(2026, 5, 1, 0, 0, 0, 0, time.UTC) }
	return f
}

func req(month string) report.CreateReportRequest {
	return report.CreateReportRequest{Month: month, Worker: "Ana", NarrativeReport: "secret text"}
}

func TestReportService_Create(t *testing.T) {
	ctx := context.Background()
	f := newFixture()

	t.Run("defaults year, writes one audit row without narrative text", func(t *testing.T) {
		got, err := f.svc.Create(ctx, alice, "10.0.0.1", req("April"))
		require.NoError(t, err)
		assert.Equal(t, 2026, got.Year)

		entries := f.audit.Entries()
		require.Len(t, entries, 1)
		e := entries[0]
		assert.Equal(t, audit.ActionCreateReport, e.Action)
		assert.Equal(t, "Report", e.TargetType)
		assert.Equal(t, "user", *e.PerformedByRole)
		assert.Equal(t, "10.0.0.1", *e.IPAddress)
		assert.Equal(t, map[string]any{"month": "April", "year": 2026, "count": 1}, e.Details)
	})

	t.Run("explicit year wins", func(t *testing.T) {
		y := 2024
		r := req("January")
		r.Year = &y
		got, err := f.svc.Create(ctx, bob, "", r)
		require.NoError(t, err)
		assert.Equal(t, 2024, got.Year)
	})

	t.Run("duplicate period is report.ErrReportExists and writes no audit row", func(t *testing.T) {
		before := len(f.audit.Entries())
		_, err := f.svc.Create(ctx, alice, "", req("April"))
		require.ErrorIs(t, err, report.ErrReportExists)
		assert.Len(t, f.audit.Entries(), before)
	})

	t.Run("audit failure fails the call", func(t *testing.T) {
		boom := errors.New("boom")
		f.audit.InsertFunc = func(context.Context, audit.Log) error { return boom }
		defer func() { f.audit.InsertFunc = nil }()
		_, err := f.svc.Create(ctx, bob, "", req("February"))
		require.ErrorIs(t, err, boom)
	})

	t.Run("unknown user", func(t *testing.T) {
		_, err := f.svc.Create(ctx, 99, "", req("March"))
		require.ErrorIs(t, err, database.ErrNotFound)
	})
}

func TestReportService_CreatePeriod(t *testing.T) {
	ctx := context.Background()
	at := func(f fixture, t time.Time) { f.svc.now = func() time.Time { return t } }

	t.Run("user may file only the previous month in Asia/Manila", func(t *testing.T) {
		f := newFixture()
		// 17:00 UTC on 31 May is already 1 June in Manila.
		at(f, time.Date(2026, 5, 31, 17, 0, 0, 0, time.UTC))
		_, err := f.svc.Create(ctx, alice, "", req("April"))
		require.ErrorIs(t, err, report.ErrPeriodNotAllowed)
		assert.Empty(t, f.audit.Entries(), "a rejected report writes no audit row")
		got, err := f.svc.Create(ctx, alice, "", req("May"))
		require.NoError(t, err)
		assert.Equal(t, 2026, got.Year)
	})

	t.Run("1 January in Manila means December of the previous year", func(t *testing.T) {
		f := newFixture()
		at(f, time.Date(2025, 12, 31, 16, 0, 0, 0, time.UTC)) // 00:00 on 1 Jan, Manila
		got, err := f.svc.Create(ctx, alice, "", req("December"))
		require.NoError(t, err)
		assert.Equal(t, 2025, got.Year, "a missing year follows the previous month")

		y := 2026
		r := req("December")
		r.Year = &y
		_, err = f.svc.Create(ctx, alice, "", r)
		require.ErrorIs(t, err, report.ErrPeriodNotAllowed, "right month, wrong year")
	})

	t.Run("a user cannot file the current month, a future month or an older year", func(t *testing.T) {
		f := newFixture() // 1 May: previous month is April
		for _, m := range []string{"May", "June", "March"} {
			_, err := f.svc.Create(ctx, alice, "", req(m))
			require.ErrorIs(t, err, report.ErrPeriodNotAllowed, m)
		}
	})

	t.Run("write_any may file late", func(t *testing.T) {
		f := newFixture()
		_, err := f.svc.Create(ctx, bob, "", req("January"))
		require.NoError(t, err)
	})
}

func TestReportService_Ownership(t *testing.T) {
	ctx := context.Background()
	f := newFixture()
	mine, err := f.svc.Create(ctx, alice, "", req("April"))
	require.NoError(t, err)
	theirs, err := f.svc.Create(ctx, bob, "", req("April"))
	require.NoError(t, err)

	t.Run("another user's report is 404 on get, update and delete", func(t *testing.T) {
		_, err := f.svc.Get(ctx, alice, theirs.ID)
		require.ErrorIs(t, err, report.ErrReportNotFound)
		_, err = f.svc.Update(ctx, alice, "", theirs.ID, report.UpdateReportRequest(req("April")))
		require.ErrorIs(t, err, report.ErrReportNotFound)
		require.ErrorIs(t, f.svc.Delete(ctx, alice, "", theirs.ID), report.ErrReportNotFound)
	})

	t.Run("missing report is the same 404", func(t *testing.T) {
		_, err := f.svc.Get(ctx, alice, 9999)
		require.ErrorIs(t, err, report.ErrReportNotFound)
	})

	t.Run("_any permissions reach other people's reports", func(t *testing.T) {
		got, err := f.svc.Get(ctx, bob, mine.ID)
		require.NoError(t, err)
		assert.Equal(t, mine.ID, got.ID)
		_, err = f.svc.Update(ctx, bob, "", mine.ID, report.UpdateReportRequest(req("April")))
		require.NoError(t, err)
	})

	t.Run("list is own-only without read_any", func(t *testing.T) {
		pg := pagination.Params{Page: 1, Limit: 10}
		got, total, err := f.svc.List(ctx, alice, report.ListReportsParams{}, pg)
		require.NoError(t, err)
		assert.EqualValues(t, 1, total)
		assert.Equal(t, mine.ID, got[0].ID)

		_, total, err = f.svc.List(ctx, bob, report.ListReportsParams{}, pg)
		require.NoError(t, err)
		assert.EqualValues(t, 2, total)
	})

	t.Run("delete by owner audits and hides the report", func(t *testing.T) {
		require.NoError(t, f.svc.Delete(ctx, alice, "", mine.ID))
		_, err := f.svc.Get(ctx, alice, mine.ID)
		require.ErrorIs(t, err, report.ErrReportNotFound)
		require.ErrorIs(t, f.svc.Delete(ctx, alice, "", mine.ID), report.ErrReportNotFound)

		entries := f.audit.Entries()
		assert.Equal(t, audit.ActionDeleteReport, entries[len(entries)-1].Action)
	})
}

func TestReportService_Update(t *testing.T) {
	ctx := context.Background()
	f := newFixture()
	v1, v2 := json.Number("1"), json.Number("2")
	first := req("April")
	first.Metrics = map[string]report.MetricWeeks{"outreach": {Week1: &v1}}
	created, err := f.svc.Create(ctx, alice, "", first)
	require.NoError(t, err)

	upd := report.UpdateReportRequest(req("April"))
	upd.Worker = "" // empty string clears the field (D-8)
	upd.Metrics = map[string]report.MetricWeeks{"training": {Week2: &v2}}
	got, err := f.svc.Update(ctx, alice, "", created.ID, upd)
	require.NoError(t, err)

	assert.Equal(t, 2026, got.Year, "missing year keeps the current one")
	assert.Empty(t, got.Worker)
	assert.NotContains(t, got.Metrics, "outreach", "metric set is replaced, not merged")
	assert.Contains(t, got.Metrics, "training")

	_, err = f.svc.Update(ctx, alice, "", 9999, upd)
	require.ErrorIs(t, err, report.ErrReportNotFound)

	f.svc.now = func() time.Time { return time.Date(2026, 6, 1, 0, 0, 0, 0, time.UTC) }
	_, err = f.svc.Create(ctx, alice, "", req("May"))
	require.NoError(t, err)
	upd.Month = "May"
	_, err = f.svc.Update(ctx, alice, "", created.ID, upd)
	require.ErrorIs(t, err, report.ErrReportExists)
}

func TestReportService_ToggleComplete(t *testing.T) {
	ctx := context.Background()
	f := newFixture()
	created, err := f.svc.Create(ctx, alice, "", req("April"))
	require.NoError(t, err)

	// bob approves alice's report; the result is metadata only, never report text
	on, err := f.svc.ToggleComplete(ctx, bob, "", created.ID)
	require.NoError(t, err)
	assert.True(t, on.Completed)
	assert.Equal(t, created.ID, on.ID)
	assert.Equal(t, bob, *on.CompletedBy)
	assert.NotNil(t, on.CompletedAt)
	body, err := json.Marshal(on)
	require.NoError(t, err)
	assert.NotContains(t, string(body), "secret text")
	assert.NotContains(t, string(body), "narrative")

	off, err := f.svc.ToggleComplete(ctx, bob, "", created.ID)
	require.NoError(t, err)
	assert.False(t, off.Completed)
	assert.Nil(t, off.CompletedBy)

	entries := f.audit.Entries()
	require.Len(t, entries, 3)
	assert.Equal(t, audit.ActionApproveReport, entries[1].Action)
	assert.Equal(t, audit.ActionUnapproveReport, entries[2].Action)

	_, err = f.svc.ToggleComplete(ctx, bob, "", 9999)
	require.ErrorIs(t, err, report.ErrReportNotFound)
}

// D-18: callers without ownership, write_any or delete_any get no free text and
// no tithes_offering; the period, status and the other 20 metrics stay.
func TestReportService_FieldVisibility(t *testing.T) {
	ctx := context.Background()
	f := newFixture()
	n := json.Number("7")
	full := req("April")
	full.Names, full.Challenges, full.PrayerRequest = "Juan", "none", "pray for Ana"
	full.Metrics = map[string]report.MetricWeeks{
		"tithes_offering": {Week1: &n},
		"outreach":        {Week1: &n},
	}
	mine, err := f.svc.Create(ctx, alice, "", full)
	require.NoError(t, err)
	pg := pagination.Params{Page: 1, Limit: 10}

	full_ := func(t *testing.T, r *report.ReportResponse) {
		t.Helper()
		assert.Equal(t, "secret text", r.NarrativeReport)
		assert.Equal(t, "Juan", r.Names)
		assert.Equal(t, "none", r.Challenges)
		assert.Equal(t, "pray for Ana", r.PrayerRequest)
		assert.Contains(t, r.Metrics, "tithes_offering")
	}
	restricted := func(t *testing.T, r *report.ReportResponse) {
		t.Helper()
		assert.Empty(t, r.NarrativeReport)
		assert.Empty(t, r.Names)
		assert.Empty(t, r.Challenges)
		assert.Empty(t, r.PrayerRequest)
		assert.NotContains(t, r.Metrics, "tithes_offering")
		assert.Contains(t, r.Metrics, "outreach", "other metrics stay visible")
		assert.Equal(t, mine.ID, r.ID)
		assert.Equal(t, "April", r.Month)
		assert.Equal(t, "Ana", r.Worker)
		assert.Equal(t, alice, r.CreatedBy)
	}

	cases := []struct {
		name string
		user int64
		full bool
	}{
		{"owner", alice, true},
		{"write_any holder", bob, true},
		{"delete_any holder", dave, true},
		{"reader with read_any only", carol, false},
	}
	for _, tc := range cases {
		check := restricted
		if tc.full {
			check = full_
		}
		t.Run(tc.name+" Get", func(t *testing.T) {
			got, err := f.svc.Get(ctx, tc.user, mine.ID)
			require.NoError(t, err)
			check(t, got)
		})
		t.Run(tc.name+" List", func(t *testing.T) {
			got, total, err := f.svc.List(ctx, tc.user, report.ListReportsParams{}, pg)
			require.NoError(t, err)
			require.EqualValues(t, 1, total)
			check(t, &got[0])
		})
	}

	t.Run("redaction does not change what is stored", func(t *testing.T) {
		_, err := f.svc.Get(ctx, carol, mine.ID)
		require.NoError(t, err)
		got, err := f.svc.Get(ctx, alice, mine.ID)
		require.NoError(t, err)
		full_(t, got)
	})
}

// Restricted readers see only the deceased status (for the Archived badge), never
// inactive or disciplinary; full readers see the raw status.
func TestActor_RedactOwnerStatus(t *testing.T) {
	deleted := time.Now()
	for _, status := range []string{"active", "inactive", "disciplinary", "deceased"} {
		t.Run(status, func(t *testing.T) {
			reader := &actor{user: &user.User{ID: carol}, perms: map[string]bool{"reports:read_any": true}}
			resp := report.ReportResponse{CreatedBy: alice, OwnerStatus: status, OwnerDeletedAt: &deleted}
			reader.redact(&resp)
			want := ""
			if status == "deceased" {
				want = "deceased"
			}
			assert.Equal(t, want, resp.OwnerStatus)
			assert.Equal(t, &deleted, resp.OwnerDeletedAt, "the Deleted user badge stays")

			admin := &actor{user: &user.User{ID: bob}, perms: map[string]bool{"reports:write_any": true}}
			resp = report.ReportResponse{CreatedBy: alice, OwnerStatus: status}
			admin.redact(&resp)
			assert.Equal(t, status, resp.OwnerStatus)
		})
	}
}

func TestReportService_ExportOne(t *testing.T) {
	ctx := context.Background()
	f := newFixture()
	r := req("April")
	r.ChurchName = "Gospel Bearer"
	rep, err := f.svc.Create(ctx, alice, "", r)
	require.NoError(t, err)
	before := len(f.audit.Entries())

	t.Run("read_any exports any report and writes one audit row", func(t *testing.T) {
		x, err := f.svc.ExportOne(ctx, bob, "10.0.0.2", rep.ID)
		require.NoError(t, err)
		assert.Equal(t, "Gospel Bearer April 2026.xlsx", x.Filename)
		assert.NotEmpty(t, x.Data)
		entries := f.audit.Entries()
		require.Len(t, entries, before+1)
		e := entries[len(entries)-1]
		assert.Equal(t, audit.ActionDownloadSingle, e.Action)
		assert.Equal(t, strconv.FormatInt(rep.ID, 10), *e.TargetID)
		assert.Equal(t, map[string]any{"month": "April", "year": 2026, "count": 1}, e.Details)
	})

	t.Run("without read_any another user's report is not found and not audited", func(t *testing.T) {
		n := len(f.audit.Entries())
		other, err := f.svc.Create(ctx, bob, "", req("January"))
		require.NoError(t, err)
		_, err = f.svc.ExportOne(ctx, alice, "", other.ID)
		require.ErrorIs(t, err, report.ErrReportNotFound)
		assert.Len(t, f.audit.Entries(), n+1) // only bob's create
	})

	t.Run("audit failure fails the export", func(t *testing.T) {
		boom := errors.New("boom")
		f.audit.InsertFunc = func(context.Context, audit.Log) error { return boom }
		defer func() { f.audit.InsertFunc = nil }()
		_, err := f.svc.ExportOne(ctx, bob, "", rep.ID)
		require.ErrorIs(t, err, boom)
	})

	t.Run("no church name falls back to the report id", func(t *testing.T) {
		plain, err := f.svc.Create(ctx, bob, "", req("February"))
		require.NoError(t, err)
		x, err := f.svc.ExportOne(ctx, bob, "", plain.ID)
		require.NoError(t, err)
		assert.Equal(t, "Report "+strconv.FormatInt(plain.ID, 10)+" February 2026.xlsx", x.Filename)
	})
}

func TestReportService_ExportMonth(t *testing.T) {
	ctx := context.Background()
	f := newFixture()
	may := report.ExportReportsRequest{Month: "May", Year: 2026}
	for _, c := range []struct {
		user   int64
		church string
	}{{alice, "zion"}, {bob, "Bethel"}, {dave, "bethel"}} {
		r := req("May")
		y := 2026
		r.Year, r.ChurchName = &y, c.church
		_, err := f.reports.Create(ctx, c.user, 2026, r)
		require.NoError(t, err)
	}

	t.Run("one sheet per report, by church then id, one audit row", func(t *testing.T) {
		x, err := f.svc.ExportMonth(ctx, bob, "10.0.0.3", may)
		require.NoError(t, err)
		assert.Equal(t, "Church Growth Program Report May 2026.xlsx", x.Filename)
		wb, err := excelize.OpenReader(bytes.NewReader(x.Data))
		require.NoError(t, err)
		defer func() { _ = wb.Close() }()
		assert.Equal(t, []string{"Bethel", "bethel (2)", "zion"}, wb.GetSheetList())

		entries := f.audit.Entries()
		require.Len(t, entries, 1)
		e := entries[0]
		assert.Equal(t, audit.ActionDownloadBulk, e.Action)
		assert.Nil(t, e.TargetID)
		assert.Equal(t, map[string]any{"month": "May", "year": 2026, "count": 3}, e.Details)
	})

	t.Run("without read_any only the caller's reports", func(t *testing.T) {
		x, err := f.svc.ExportMonth(ctx, alice, "", may)
		require.NoError(t, err)
		wb, err := excelize.OpenReader(bytes.NewReader(x.Data))
		require.NoError(t, err)
		defer func() { _ = wb.Close() }()
		assert.Equal(t, []string{"zion"}, wb.GetSheetList())
	})

	t.Run("empty month is ErrNoReports and not audited", func(t *testing.T) {
		n := len(f.audit.Entries())
		_, err := f.svc.ExportMonth(ctx, bob, "", report.ExportReportsRequest{Month: "June", Year: 2026})
		require.ErrorIs(t, err, report.ErrNoReports)
		assert.Len(t, f.audit.Entries(), n)
	})

	t.Run("an emoji church name still exports every sheet", func(t *testing.T) {
		for _, c := range []struct {
			user   int64
			church string
		}{{alice, strings.Repeat("🙏", 20)}, {bob, "Bethel"}} {
			r := req("July")
			y := 2026
			r.Year, r.ChurchName = &y, c.church
			_, err := f.reports.Create(ctx, c.user, 2026, r)
			require.NoError(t, err)
		}
		x, err := f.svc.ExportMonth(ctx, bob, "", report.ExportReportsRequest{Month: "July", Year: 2026})
		require.NoError(t, err)
		wb, err := excelize.OpenReader(bytes.NewReader(x.Data))
		require.NoError(t, err)
		defer func() { _ = wb.Close() }()
		assert.Equal(t, []string{"Bethel", strings.Repeat("🙏", 15)}, wb.GetSheetList())
	})

	t.Run("more than the cap is ErrTooManyReports and not audited", func(t *testing.T) {
		for u := int64(100); u < 100+report.MaxExportReports; u++ {
			_, err := f.reports.Create(ctx, u, 2025, req("March"))
			require.NoError(t, err)
		}
		n := len(f.audit.Entries())
		_, err := f.svc.ExportMonth(ctx, bob, "", report.ExportReportsRequest{Month: "March", Year: 2025})
		require.NoError(t, err, "exactly the cap is allowed")
		_, err = f.reports.Create(ctx, 999, 2025, req("March"))
		require.NoError(t, err)
		_, err = f.svc.ExportMonth(ctx, bob, "", report.ExportReportsRequest{Month: "March", Year: 2025})
		require.ErrorIs(t, err, report.ErrTooManyReports)
		assert.Len(t, f.audit.Entries(), n+1)
	})
}

func TestReportService_Preview(t *testing.T) {
	ctx := context.Background()
	f := newFixture()
	for _, c := range []struct {
		user   int64
		church string
	}{{alice, "zion"}, {bob, "Bethel"}, {dave, "bethel"}} {
		r := req("May")
		y := 2026
		r.Year, r.ChurchName = &y, c.church
		_, err := f.reports.Create(ctx, c.user, 2026, r)
		require.NoError(t, err)
	}
	r := req("April")
	r.ChurchName = "Gospel Bearer"
	rep, err := f.svc.Create(ctx, alice, "", r)
	require.NoError(t, err)
	before := len(f.audit.Entries())
	names := func(p *report.ExcelPreview) []string {
		var out []string
		for _, s := range p.Sheets {
			out = append(out, s.Name)
		}
		return out
	}

	t.Run("one report", func(t *testing.T) {
		p, err := f.svc.Preview(ctx, bob, report.ExcelPreviewRequest{ReportID: rep.ID})
		require.NoError(t, err)
		assert.Equal(t, 1, p.Count)
		assert.Equal(t, []string{"Gospel Bearer"}, names(p))
		assert.Equal(t, "Gospel Bearer", p.Sheets[0].Rows[6][1]) // B7
	})

	t.Run("a month, in the export's sheet order", func(t *testing.T) {
		p, err := f.svc.Preview(ctx, bob, report.ExcelPreviewRequest{Month: "May", Year: 2026})
		require.NoError(t, err)
		assert.Equal(t, 3, p.Count)
		assert.Equal(t, []string{"Bethel", "bethel (2)", "zion"}, names(p))
	})

	t.Run("a missing or deleted report is not found", func(t *testing.T) {
		_, err := f.svc.Preview(ctx, bob, report.ExcelPreviewRequest{ReportID: 9999})
		require.ErrorIs(t, err, report.ErrReportNotFound)
		gone, err := f.svc.Create(ctx, bob, "", req("January"))
		require.NoError(t, err)
		require.NoError(t, f.svc.Delete(ctx, bob, "", gone.ID))
		_, err = f.svc.Preview(ctx, bob, report.ExcelPreviewRequest{ReportID: gone.ID})
		require.ErrorIs(t, err, report.ErrReportNotFound)
		before = len(f.audit.Entries())
	})

	t.Run("an empty month is ErrNoReports", func(t *testing.T) {
		_, err := f.svc.Preview(ctx, bob, report.ExcelPreviewRequest{Month: "June", Year: 2026})
		require.ErrorIs(t, err, report.ErrNoReports)
	})

	t.Run("more than the cap is ErrTooManyReports", func(t *testing.T) {
		for u := int64(100); u <= 100+report.MaxExportReports; u++ {
			_, err := f.reports.Create(ctx, u, 2025, req("March"))
			require.NoError(t, err)
		}
		_, err := f.svc.Preview(ctx, bob, report.ExcelPreviewRequest{Month: "March", Year: 2025})
		require.ErrorIs(t, err, report.ErrTooManyReports)
	})

	assert.Len(t, f.audit.Entries(), before, "a preview never writes an audit row")
}
