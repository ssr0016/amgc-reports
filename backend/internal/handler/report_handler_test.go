package handler

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
	"github.com/ssr0016/pastoral-reports/backend/internal/model"
	"github.com/ssr0016/pastoral-reports/backend/internal/repository"
	"github.com/ssr0016/pastoral-reports/backend/internal/request"
	"github.com/ssr0016/pastoral-reports/backend/internal/service"
	"github.com/ssr0016/pastoral-reports/backend/internal/validator"
	"github.com/ssr0016/pastoral-reports/backend/pkg/pagination"
)

// fakeReportSvc returns err from every method, and records what it was called with.
type fakeReportSvc struct {
	err      error
	gotUser  int64
	gotIP    string
	gotID    int64
	gotList  model.ListReportsParams
	gotPage  pagination.Params
	listData []model.ReportResponse
	total    int64
}

func (f *fakeReportSvc) Create(_ context.Context, uid int64, ip string, _ model.CreateReportRequest) (*model.ReportResponse, error) {
	f.gotUser, f.gotIP = uid, ip
	return &model.ReportResponse{ID: 1}, f.err
}

func (f *fakeReportSvc) Get(_ context.Context, uid, id int64) (*model.ReportResponse, error) {
	f.gotUser, f.gotID = uid, id
	return &model.ReportResponse{ID: id}, f.err
}

func (f *fakeReportSvc) List(_ context.Context, uid int64, p model.ListReportsParams, pg pagination.Params) ([]model.ReportResponse, int64, error) {
	f.gotUser, f.gotList, f.gotPage = uid, p, pg
	return f.listData, f.total, f.err
}

func (f *fakeReportSvc) Update(_ context.Context, uid int64, _ string, id int64, _ model.UpdateReportRequest) (*model.ReportResponse, error) {
	f.gotUser, f.gotID = uid, id
	return &model.ReportResponse{ID: id}, f.err
}

func (f *fakeReportSvc) Delete(_ context.Context, uid int64, _ string, id int64) error {
	f.gotUser, f.gotID = uid, id
	return f.err
}

func (f *fakeReportSvc) ToggleComplete(_ context.Context, uid int64, _ string, id int64) (*model.ReportMetadata, error) {
	f.gotUser, f.gotID = uid, id
	return &model.ReportMetadata{ID: id, Completed: true}, f.err
}

// serveReport routes one request through the handler with an optional session user.
func serveReport(t *testing.T, svc ReportServiceInterface, userID int64, method, target, body string) (*httptest.ResponseRecorder, errorBody) {
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
	}
	errs := []struct {
		name   string
		err    error
		status int
		code   string
	}{
		{"not found", fmt.Errorf("x: %w", service.ErrReportNotFound), 404, "NOT_FOUND"},
		{"exists", fmt.Errorf("x: %w", service.ErrReportExists), 409, "CONFLICT"},
		{"unknown user", fmt.Errorf("user: %w", repository.ErrNotFound), 401, "UNAUTHORIZED"},
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
	f := &fakeReportSvc{err: fmt.Errorf("x: %w", service.ErrPeriodNotAllowed)}
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
		f := &fakeReportSvc{listData: []model.ReportResponse{{ID: 1}}, total: 41}
		rec, _ := serveReport(t, f, 5, http.MethodGet, "/reports?year=2026&month=May&worker=Ana&area=N&church=C&page=2&limit=20", "")
		var got struct {
			Data []model.ReportResponse `json:"data"`
			Meta pagination.Meta        `json:"meta"`
		}
		if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
			t.Fatal(err)
		}
		if rec.Code != 200 || len(got.Data) != 1 || got.Meta.Total != 41 || got.Meta.TotalPages != 3 || got.Meta.Page != 2 {
			t.Errorf("status=%d body=%s", rec.Code, rec.Body.String())
		}
		want := model.ListReportsParams{Year: 2026, Month: "May", Worker: "Ana", Area: "N", Church: "C"}
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
