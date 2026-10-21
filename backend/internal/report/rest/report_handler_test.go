package rest

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/labstack/echo/v4"

	"github.com/ssr0016/pastoral-reports/backend/internal/apperror"
	"github.com/ssr0016/pastoral-reports/backend/internal/database"
	"github.com/ssr0016/pastoral-reports/backend/internal/report"
	"github.com/ssr0016/pastoral-reports/backend/internal/request"
	"github.com/ssr0016/pastoral-reports/backend/internal/validator"
	"github.com/ssr0016/pastoral-reports/backend/pkg/pagination"
)

// fakeReportSvc returns err from every method, and records what it was called with.
type fakeReportSvc struct {
	err       error
	gotUser   int64
	gotIP     string
	gotID     int64
	gotList   report.ListReportsParams
	gotPage   pagination.Params
	listData  []report.ReportResponse
	gotExport report.ExportReportsRequest
	total     int64
}

func (f *fakeReportSvc) Create(_ context.Context, uid int64, ip string, _ report.CreateReportRequest) (*report.ReportResponse, error) {
	f.gotUser, f.gotIP = uid, ip
	return &report.ReportResponse{ID: 1}, f.err
}

func (f *fakeReportSvc) Get(_ context.Context, uid, id int64) (*report.ReportResponse, error) {
	f.gotUser, f.gotID = uid, id
	return &report.ReportResponse{ID: id}, f.err
}

func (f *fakeReportSvc) List(_ context.Context, uid int64, p report.ListReportsParams, pg pagination.Params) ([]report.ReportResponse, int64, error) {
	f.gotUser, f.gotList, f.gotPage = uid, p, pg
	return f.listData, f.total, f.err
}

func (f *fakeReportSvc) Update(_ context.Context, uid int64, _ string, id int64, _ report.UpdateReportRequest) (*report.ReportResponse, error) {
	f.gotUser, f.gotID = uid, id
	return &report.ReportResponse{ID: id}, f.err
}

func (f *fakeReportSvc) Delete(_ context.Context, uid int64, _ string, id int64) error {
	f.gotUser, f.gotID = uid, id
	return f.err
}

func (f *fakeReportSvc) ToggleComplete(_ context.Context, uid int64, _ string, id int64) (*report.ReportMetadata, error) {
	f.gotUser, f.gotID = uid, id
	return &report.ReportMetadata{ID: id, Completed: true}, f.err
}

func (f *fakeReportSvc) ExportOne(_ context.Context, uid int64, ip string, id int64) (*report.Export, error) {
	f.gotUser, f.gotIP, f.gotID = uid, ip, id
	return &report.Export{Filename: "Café May 2026.xlsx", Data: []byte("PK")}, f.err
}

func (f *fakeReportSvc) ExportMonth(_ context.Context, uid int64, _ string, req report.ExportReportsRequest) (*report.Export, error) {
	f.gotUser, f.gotExport = uid, req
	return &report.Export{Filename: "Church Growth Program Report May 2026.xlsx", Data: []byte("PK")}, f.err
}

// serveReport routes one request through the handler with an optional session user.
func serveReport(t *testing.T, svc report.Service, userID int64, method, target, body string) (*httptest.ResponseRecorder, errorBody) {
	t.Helper()
	h := NewReportHandler(svc)

	e := echo.New()
	e.Validator = validator.New()
	e.HTTPErrorHandler = apperror.ErrorHandler
	e.Use(func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			if userID != 0 {
				c.SetRequest(c.Request().WithContext(request.WithUser(c.Request().Context(), request.User{ID: userID})))
			}
			return next(c)
		}
	})
	e.POST("/reports", h.CreateReport)
	e.GET("/reports", h.ListReports)
	e.GET("/reports/:id", h.GetReport)
	e.PUT("/reports/:id", h.UpdateReport)
	e.DELETE("/reports/:id", h.SoftDeleteReport)
	e.PATCH("/reports/:id/complete", h.ToggleComplete)
	e.POST("/admin/reports/excel", h.ExportReports)
	e.POST("/admin/reports/:id/excel", h.ExportReport)

	req := httptest.NewRequestWithContext(context.Background(), method, target, strings.NewReader(body))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	var eb errorBody
	if rec.Code >= 400 {
		if err := json.Unmarshal(rec.Body.Bytes(), &eb); err != nil {
			t.Fatalf("decode %q: %v", rec.Body.String(), err)
		}
	}
	return rec, eb
}

const validBody = `{"month":"January","metrics":{"outreach":{"week1":1.50}}}`

func TestReportHandler_ErrorMapping(t *testing.T) {
	endpoints := []struct{ name, method, target, body string }{
		{"create", http.MethodPost, "/reports", validBody},
		{"get", http.MethodGet, "/reports/7", ""},
		{"list", http.MethodGet, "/reports", ""},
		{"update", http.MethodPut, "/reports/7", validBody},
		{"delete", http.MethodDelete, "/reports/7", ""},
		{"toggle", http.MethodPatch, "/reports/7/complete", ""},
		{"export one", http.MethodPost, "/admin/reports/7/excel", ""},
		{"export month", http.MethodPost, "/admin/reports/excel", `{"month":"May","year":2026}`},
	}
	errs := []struct {
		name   string
		err    error
		status int
		code   string
	}{
		{"not found", fmt.Errorf("x: %w", report.ErrReportNotFound), 404, "NOT_FOUND"},
		{"exists", fmt.Errorf("x: %w", report.ErrReportExists), 409, "CONFLICT"},
		{"unknown user", fmt.Errorf("user: %w", database.ErrNotFound), 401, "UNAUTHORIZED"},
		{"other", errors.New("connection refused"), 500, "INTERNAL_ERROR"},
	}
	for _, ep := range endpoints {
		for _, tc := range errs {
			t.Run(ep.name+"/"+tc.name, func(t *testing.T) {
				rec, body := serveReport(t, &fakeReportSvc{err: tc.err}, 5, ep.method, ep.target, ep.body)
				if rec.Code != tc.status || body.Error != tc.code {
					t.Errorf("got %d %q, want %d %q", rec.Code, body.Error, tc.status, tc.code)
				}
			})
		}
	}
}

// The form maps details["month"] onto the month field.
func TestReportHandler_PeriodNotAllowedIs422WithMonthDetail(t *testing.T) {
	f := &fakeReportSvc{err: fmt.Errorf("x: %w", report.ErrPeriodNotAllowed)}
	rec, body := serveReport(t, f, 5, http.MethodPost, "/reports", validBody)
	if rec.Code != 422 || body.Error != "VALIDATION_ERROR" {
		t.Errorf("got %d %q, want 422 VALIDATION_ERROR", rec.Code, body.Error)
	}
	if !strings.Contains(rec.Body.String(), `"month":"Reports can only be created for the previous month."`) {
		t.Errorf("body = %s", rec.Body.String())
	}
}

func TestReportHandler_NoSessionIs401(t *testing.T) {
	rec, _ := serveReport(t, &fakeReportSvc{}, 0, http.MethodGet, "/reports/7", "")
	if rec.Code != http.StatusUnauthorized {
		t.Errorf("status = %d, want 401", rec.Code)
	}
}

func TestReportHandler_Success(t *testing.T) {
	t.Run("create is 201 and passes user and ip", func(t *testing.T) {
		f := &fakeReportSvc{}
		rec, _ := serveReport(t, f, 5, http.MethodPost, "/reports", validBody)
		if rec.Code != http.StatusCreated || f.gotUser != 5 || f.gotIP == "" {
			t.Errorf("status=%d user=%d ip=%q", rec.Code, f.gotUser, f.gotIP)
		}
	})
	t.Run("get passes id", func(t *testing.T) {
		f := &fakeReportSvc{}
		rec, _ := serveReport(t, f, 5, http.MethodGet, "/reports/7", "")
		if rec.Code != http.StatusOK || f.gotID != 7 {
			t.Errorf("status=%d id=%d", rec.Code, f.gotID)
		}
	})
	t.Run("update is 200", func(t *testing.T) {
		rec, _ := serveReport(t, &fakeReportSvc{}, 5, http.MethodPut, "/reports/7", validBody)
		if rec.Code != http.StatusOK {
			t.Errorf("status=%d", rec.Code)
		}
	})
	t.Run("delete is 204", func(t *testing.T) {
		rec, _ := serveReport(t, &fakeReportSvc{}, 5, http.MethodDelete, "/reports/7", "")
		if rec.Code != http.StatusNoContent {
			t.Errorf("status=%d", rec.Code)
		}
	})
	t.Run("toggle returns metadata only", func(t *testing.T) {
		rec, _ := serveReport(t, &fakeReportSvc{}, 5, http.MethodPatch, "/reports/7/complete", "")
		if rec.Code != http.StatusOK || strings.Contains(rec.Body.String(), "narrative") {
			t.Errorf("status=%d body=%s", rec.Code, rec.Body.String())
		}
		if got := rec.Body.String(); !strings.Contains(got, `"completed":true`) {
			t.Errorf("body = %s", got)
		}
	})
	t.Run("list is {data, meta} with filters and paging", func(t *testing.T) {
		f := &fakeReportSvc{listData: []report.ReportResponse{{ID: 1}}, total: 41}
		rec, _ := serveReport(t, f, 5, http.MethodGet, "/reports?year=2026&month=May&worker=Ana&area=N&church=C&page=2&limit=20", "")
		var got struct {
			Data []report.ReportResponse `json:"data"`
			Meta pagination.Meta         `json:"meta"`
		}
		if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
			t.Fatal(err)
		}
		if rec.Code != 200 || len(got.Data) != 1 || got.Meta.Total != 41 || got.Meta.TotalPages != 3 || got.Meta.Page != 2 {
			t.Errorf("status=%d body=%s", rec.Code, rec.Body.String())
		}
		want := report.ListReportsParams{Year: 2026, Month: "May", Worker: "Ana", Area: "N", Church: "C"}
		if f.gotList != want {
			t.Errorf("params = %+v, want %+v", f.gotList, want)
		}
	})
	t.Run("empty list is [] not null", func(t *testing.T) {
		rec, _ := serveReport(t, &fakeReportSvc{}, 5, http.MethodGet, "/reports", "")
		if !strings.Contains(rec.Body.String(), `"data":[]`) {
			t.Errorf("body = %s", rec.Body.String())
		}
	})
}

func TestReportHandler_BadInput(t *testing.T) {
	tests := []struct {
		name, method, target, body string
		status                     int
	}{
		{"bad id", http.MethodGet, "/reports/abc", "", 400},
		{"zero id", http.MethodDelete, "/reports/0", "", 400},
		{"bad year", http.MethodGet, "/reports?year=abc", "", 400},
		{"malformed json", http.MethodPost, "/reports", "{", 400},
		{"unknown month", http.MethodPost, "/reports", `{"month":"Smarch"}`, 422},
		{"unknown metric key", http.MethodPost, "/reports", `{"month":"May","metrics":{"nope":{"week1":1}}}`, 422},
		{"three decimals", http.MethodPost, "/reports", `{"month":"May","metrics":{"outreach":{"week1":1.234}}}`, 422},
		{"negative", http.MethodPut, "/reports/7", `{"month":"May","metrics":{"outreach":{"week1":-1}}}`, 422},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			f := &fakeReportSvc{}
			rec, _ := serveReport(t, f, 5, tt.method, tt.target, tt.body)
			if rec.Code != tt.status {
				t.Errorf("status = %d, want %d (%s)", rec.Code, tt.status, rec.Body.String())
			}
			if f.gotUser != 0 {
				t.Error("service was called for invalid input")
			}
		})
	}
}

// errorBody is the JSON error body apperror writes.
type errorBody struct {
	Error   string            `json:"error"`
	Message string            `json:"message"`
	Details map[string]string `json:"details"`
}

func TestReportHandler_ExportReport(t *testing.T) {
	f := &fakeReportSvc{}
	rec, _ := serveReport(t, f, 5, http.MethodPost, "/admin/reports/7/excel", "")
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	for header, want := range map[string]string{
		echo.HeaderContentType:        "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet",
		echo.HeaderContentDisposition: `attachment; filename="Caf_ May 2026.xlsx"; filename*=UTF-8''Caf%C3%A9%20May%202026.xlsx`,
		"Cache-Control":               "no-store",
	} {
		if got := rec.Header().Get(header); got != want {
			t.Errorf("%s = %q, want %q", header, got, want)
		}
	}
	if rec.Body.String() != "PK" || f.gotUser != 5 || f.gotID != 7 {
		t.Errorf("body %q, user %d, id %d", rec.Body.String(), f.gotUser, f.gotID)
	}

	if rec, body := serveReport(t, f, 5, http.MethodPost, "/admin/reports/abc/excel", ""); rec.Code != 400 || body.Error != "BAD_REQUEST" {
		t.Errorf("bad id: got %d %q, want 400 BAD_REQUEST", rec.Code, body.Error)
	}
	if rec, _ := serveReport(t, f, 0, http.MethodPost, "/admin/reports/7/excel", ""); rec.Code != 401 {
		t.Errorf("no session: got %d, want 401", rec.Code)
	}
}

func TestReportHandler_ExportReports(t *testing.T) {
	const may = `{"month":"May","year":2026}`
	f := &fakeReportSvc{}
	rec, _ := serveReport(t, f, 5, http.MethodPost, "/admin/reports/excel", may)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	if want := (report.ExportReportsRequest{Month: "May", Year: 2026}); f.gotExport != want {
		t.Errorf("request = %+v, want %+v", f.gotExport, want)
	}
	if got := rec.Header().Get(echo.HeaderContentDisposition); !strings.Contains(got, `filename="Church Growth Program Report May 2026.xlsx"`) {
		t.Errorf("Content-Disposition = %q", got)
	}

	for _, tc := range []struct{ name, body, field string }{
		{"missing month", `{"year":2026}`, "month"},
		{"bad month", `{"month":"may","year":2026}`, "month"},
		{"missing year", `{"month":"May"}`, "year"},
		{"year too low", `{"month":"May","year":1999}`, "year"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			rec, body := serveReport(t, &fakeReportSvc{}, 5, http.MethodPost, "/admin/reports/excel", tc.body)
			if _, ok := body.Details[tc.field]; rec.Code != 422 || !ok {
				t.Errorf("got %d %v, want 422 on %s", rec.Code, body.Details, tc.field)
			}
		})
	}

	for _, tc := range []struct {
		err    error
		status int
		msg    string
	}{
		{report.ErrNoReports, 404, "No reports for that month."},
		{report.ErrTooManyReports, 422, "A bulk export holds at most 100 reports."},
	} {
		rec, body := serveReport(t, &fakeReportSvc{err: tc.err}, 5, http.MethodPost, "/admin/reports/excel", may)
		if rec.Code != tc.status || body.Message != tc.msg {
			t.Errorf("%v: got %d %q, want %d %q", tc.err, rec.Code, body.Message, tc.status, tc.msg)
		}
	}
}
