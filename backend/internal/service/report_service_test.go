package service

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ssr0016/pastoral-reports/backend/internal/model"
	"github.com/ssr0016/pastoral-reports/backend/internal/repository"
	"github.com/ssr0016/pastoral-reports/backend/pkg/pagination"
)

// fakeTx runs fn directly. The in-memory mocks have no transaction to roll back.
type fakeTx struct{}

func (fakeTx) WithTx(_ context.Context, fn func(repository.Querier) error) error { return fn(nil) }

// stubRoles serves permissions by role ID; other methods panic if called.
type stubRoles struct {
	repository.RoleRepository
	perms map[int64][]string
}

func (s stubRoles) GetPermissions(_ context.Context, roleID int64) ([]model.Permission, error) {
	var out []model.Permission
	for _, n := range s.perms[roleID] {
		out = append(out, model.Permission{Name: n})
	}
	return out, nil
}

const (
	alice = int64(1) // plain user: own reports only
	bob   = int64(2) // editor: _any + approve
	roleU = int64(10)
	roleE = int64(20)
)

type fixture struct {
	svc     *ReportService
	reports *repository.MockReportRepo
	audit   *repository.MockAuditRepo
}

func newFixture() fixture {
	users := repository.NewMockUserRepo()
	users.GetWithRoleFunc = func(_ context.Context, id int64) (*model.User, error) {
		switch id {
		case alice:
			return &model.User{ID: alice, Name: "Alice", RoleID: roleU, Role: &model.Role{Name: "user"}}, nil
		case bob:
			return &model.User{ID: bob, Name: "Bob", RoleID: roleE, Role: &model.Role{Name: "editor"}}, nil
		}
		return nil, nil
	}
	roles := stubRoles{perms: map[int64][]string{
		roleU: {"reports:read", "reports:write", "reports:delete"},
		roleE: {"reports:read", "reports:write", "reports:delete", "reports:read_any", "reports:write_any", "reports:delete_any", "reports:approve"},
	}}
	f := fixture{reports: repository.NewMockReportRepo(), audit: repository.NewMockAuditRepo()}
	f.svc = NewReportService(f.reports, f.audit, fakeTx{}, nil, users, roles)
	f.svc.now = func() time.Time { return time.Date(2026, 5, 1, 0, 0, 0, 0, time.UTC) }
	return f
}

func req(month string) model.CreateReportRequest {
	return model.CreateReportRequest{Month: month, Worker: "Ana", NarrativeReport: "secret text"}
}

func TestReportService_Create(t *testing.T) {
	ctx := context.Background()
	f := newFixture()

	t.Run("defaults year, writes one audit row without narrative text", func(t *testing.T) {
		got, err := f.svc.Create(ctx, alice, "10.0.0.1", req("January"))
		require.NoError(t, err)
		assert.Equal(t, 2026, got.Year)

		entries := f.audit.Entries()
		require.Len(t, entries, 1)
		e := entries[0]
		assert.Equal(t, model.AuditCreateReport, e.Action)
		assert.Equal(t, "Report", e.TargetType)
		assert.Equal(t, "user", *e.PerformedByRole)
		assert.Equal(t, "10.0.0.1", *e.IPAddress)
		assert.Equal(t, map[string]any{"month": "January", "year": 2026, "count": 1}, e.Details)
	})

	t.Run("explicit year wins", func(t *testing.T) {
		y := 2024
		r := req("January")
		r.Year = &y
		got, err := f.svc.Create(ctx, alice, "", r)
		require.NoError(t, err)
		assert.Equal(t, 2024, got.Year)
	})

	t.Run("duplicate period is ErrReportExists and writes no audit row", func(t *testing.T) {
		before := len(f.audit.Entries())
		_, err := f.svc.Create(ctx, alice, "", req("January"))
		require.ErrorIs(t, err, ErrReportExists)
		assert.Len(t, f.audit.Entries(), before)
	})

	t.Run("audit failure fails the call", func(t *testing.T) {
		boom := errors.New("boom")
		f.audit.InsertFunc = func(context.Context, repository.Querier, model.AuditLog) error { return boom }
		defer func() { f.audit.InsertFunc = nil }()
		_, err := f.svc.Create(ctx, alice, "", req("February"))
		require.ErrorIs(t, err, boom)
	})

	t.Run("unknown user", func(t *testing.T) {
		_, err := f.svc.Create(ctx, 99, "", req("March"))
		require.ErrorIs(t, err, repository.ErrNotFound)
	})
}

func TestReportService_Ownership(t *testing.T) {
	ctx := context.Background()
	f := newFixture()
	mine, err := f.svc.Create(ctx, alice, "", req("January"))
	require.NoError(t, err)
	theirs, err := f.svc.Create(ctx, bob, "", req("January"))
	require.NoError(t, err)

	t.Run("another user's report is 404 on get, update and delete", func(t *testing.T) {
		_, err := f.svc.Get(ctx, alice, theirs.ID)
		require.ErrorIs(t, err, ErrReportNotFound)
		_, err = f.svc.Update(ctx, alice, "", theirs.ID, model.UpdateReportRequest(req("January")))
		require.ErrorIs(t, err, ErrReportNotFound)
		require.ErrorIs(t, f.svc.Delete(ctx, alice, "", theirs.ID), ErrReportNotFound)
	})

	t.Run("missing report is the same 404", func(t *testing.T) {
		_, err := f.svc.Get(ctx, alice, 9999)
		require.ErrorIs(t, err, ErrReportNotFound)
	})

	t.Run("_any permissions reach other people's reports", func(t *testing.T) {
		got, err := f.svc.Get(ctx, bob, mine.ID)
		require.NoError(t, err)
		assert.Equal(t, mine.ID, got.ID)
		_, err = f.svc.Update(ctx, bob, "", mine.ID, model.UpdateReportRequest(req("January")))
		require.NoError(t, err)
	})

	t.Run("list is own-only without read_any", func(t *testing.T) {
		pg := pagination.Params{Page: 1, Limit: 10}
		got, total, err := f.svc.List(ctx, alice, model.ListReportsParams{}, pg)
		require.NoError(t, err)
		assert.EqualValues(t, 1, total)
		assert.Equal(t, mine.ID, got[0].ID)

		_, total, err = f.svc.List(ctx, bob, model.ListReportsParams{}, pg)
		require.NoError(t, err)
		assert.EqualValues(t, 2, total)
	})

	t.Run("delete by owner audits and hides the report", func(t *testing.T) {
		require.NoError(t, f.svc.Delete(ctx, alice, "", mine.ID))
		_, err := f.svc.Get(ctx, alice, mine.ID)
		require.ErrorIs(t, err, ErrReportNotFound)
		require.ErrorIs(t, f.svc.Delete(ctx, alice, "", mine.ID), ErrReportNotFound)

		entries := f.audit.Entries()
		assert.Equal(t, model.AuditDeleteReport, entries[len(entries)-1].Action)
	})
}

func TestReportService_Update(t *testing.T) {
	ctx := context.Background()
	f := newFixture()
	v1, v2 := json.Number("1"), json.Number("2")
	first := req("January")
	first.Metrics = map[string]model.MetricWeeks{"outreach": {Week1: &v1}}
	created, err := f.svc.Create(ctx, alice, "", first)
	require.NoError(t, err)

	upd := model.UpdateReportRequest(req("January"))
	upd.Worker = "" // empty string clears the field (D-8)
	upd.Metrics = map[string]model.MetricWeeks{"training": {Week2: &v2}}
	got, err := f.svc.Update(ctx, alice, "", created.ID, upd)
	require.NoError(t, err)

	assert.Equal(t, 2026, got.Year, "missing year keeps the current one")
	assert.Empty(t, got.Worker)
	assert.NotContains(t, got.Metrics, "outreach", "metric set is replaced, not merged")
	assert.Contains(t, got.Metrics, "training")

	_, err = f.svc.Update(ctx, alice, "", 9999, upd)
	require.ErrorIs(t, err, ErrReportNotFound)

	_, err = f.svc.Create(ctx, alice, "", req("February"))
	require.NoError(t, err)
	upd.Month = "February"
	_, err = f.svc.Update(ctx, alice, "", created.ID, upd)
	require.ErrorIs(t, err, ErrReportExists)
}

func TestReportService_ToggleComplete(t *testing.T) {
	ctx := context.Background()
	f := newFixture()
	created, err := f.svc.Create(ctx, alice, "", req("January"))
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
	assert.Equal(t, model.AuditApproveReport, entries[1].Action)
	assert.Equal(t, model.AuditUnapproveReport, entries[2].Action)

	_, err = f.svc.ToggleComplete(ctx, bob, "", 9999)
	require.ErrorIs(t, err, ErrReportNotFound)
}
