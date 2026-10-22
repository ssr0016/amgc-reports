package rest

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/labstack/echo/v4"

	"github.com/ssr0016/pastoral-reports/backend/internal/apperror"
	"github.com/ssr0016/pastoral-reports/backend/internal/audit"
	"github.com/ssr0016/pastoral-reports/backend/pkg/pagination"
)

// fakeAuditSvc records the query and page it was asked for.
type fakeAuditSvc struct {
	err   error
	calls int
	q     audit.ListQuery
	pg    pagination.Params
}

func (f *fakeAuditSvc) List(_ context.Context, q audit.ListQuery, pg pagination.Params) ([]audit.Log, int64, error) {
	f.calls++
	f.q, f.pg = q, pg
	name, ip, id := "Ana", "10.0.0.9", "7"
	return []audit.Log{{
		ID: 1, PerformedByName: &name, Action: audit.ActionLogin, TargetType: audit.TargetTypeUser,
		TargetID: &id, TargetName: &name, IPAddress: &ip,
		CreatedAt: time.Date(2026, 10, 1, 8, 0, 0, 0, time.UTC),
	}}, 1, f.err
}

func serveAudit(t *testing.T, svc audit.Service, target string) *httptest.ResponseRecorder {
	t.Helper()
	e := echo.New()
	e.HTTPErrorHandler = apperror.ErrorHandler
	e.GET("/admin/audit-logs", NewAuditHandler(svc).List)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, httptest.NewRequestWithContext(context.Background(), http.MethodGet, target, nil))
	return rec
}

func TestAuditHandler_List(t *testing.T) {
	t.Run("passes the filters and defaults to 50 per page", func(t *testing.T) {
		svc := &fakeAuditSvc{}
		rec := serveAudit(t, svc, "/admin/audit-logs?month=October&year=2026&action=LOGIN")
		if rec.Code != http.StatusOK {
			t.Fatalf("status = %d, body %s", rec.Code, rec.Body)
		}
		if svc.q.Month != "October" || svc.q.Year != 2026 || svc.q.Action == nil || *svc.q.Action != "LOGIN" {
			t.Errorf("query = %+v", svc.q)
		}
		if svc.pg.Limit != 50 || svc.pg.Page != 1 {
			t.Errorf("page = %+v, want page 1 of 50", svc.pg)
		}
	})

	t.Run("an explicit limit is kept, no action means all", func(t *testing.T) {
		svc := &fakeAuditSvc{}
		serveAudit(t, svc, "/admin/audit-logs?month=October&year=2026&page=2&limit=10")
		if svc.pg.Limit != 10 || svc.pg.Page != 2 || svc.q.Action != nil {
			t.Errorf("page = %+v action = %v", svc.pg, svc.q.Action)
		}
	})

	t.Run("answers the page with no IP address", func(t *testing.T) {
		rec := serveAudit(t, &fakeAuditSvc{}, "/admin/audit-logs?month=October&year=2026")
		if strings.Contains(rec.Body.String(), "10.0.0.9") || strings.Contains(rec.Body.String(), "ip_address") {
			t.Fatalf("the IP address leaked: %s", rec.Body)
		}
		var got struct {
			Data []map[string]any `json:"data"`
			Meta pagination.Meta  `json:"meta"`
		}
		if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
			t.Fatal(err)
		}
		if got.Meta.Total != 1 || got.Meta.Limit != 50 || len(got.Data) != 1 {
			t.Fatalf("body = %s", rec.Body)
		}
		row := got.Data[0]
		if row["action"] != "LOGIN" || row["target_name"] != "Ana" || row["details"] == nil {
			t.Errorf("row = %v", row)
		}
	})

	tests := []struct {
		name, target string
		svcErr       error
		wantStatus   int
		wantField    string
	}{
		{"missing year", "/admin/audit-logs?month=October", audit.ErrPeriod, http.StatusUnprocessableEntity, "month"},
		{"year not a number", "/admin/audit-logs?month=October&year=abc", nil, http.StatusBadRequest, ""},
		{"bad period", "/admin/audit-logs?month=Smarch&year=2026", audit.ErrPeriod, http.StatusUnprocessableEntity, "month"},
		{"bad action", "/admin/audit-logs?month=May&year=2026&action=NOPE", audit.ErrAction, http.StatusUnprocessableEntity, "action"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rec := serveAudit(t, &fakeAuditSvc{err: tt.svcErr}, tt.target)
			if rec.Code != tt.wantStatus {
				t.Fatalf("status = %d, want %d (body %s)", rec.Code, tt.wantStatus, rec.Body)
			}
			if tt.wantField != "" && !strings.Contains(rec.Body.String(), `"`+tt.wantField+`"`) {
				t.Errorf("body %s does not name the field %q", rec.Body, tt.wantField)
			}
		})
	}
}
