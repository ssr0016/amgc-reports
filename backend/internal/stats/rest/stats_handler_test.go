package rest

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/labstack/echo/v4"

	"github.com/ssr0016/pastoral-reports/backend/internal/apperror"
	"github.com/ssr0016/pastoral-reports/backend/internal/stats"
)

// errorBody is the error envelope apperror writes.
type errorBody struct {
	Error   string `json:"error"`
	Message string `json:"message"`
}

// fakeStatsSvc records the period it was asked for and answers with fixed counts.
type fakeStatsSvc struct {
	err   error
	calls int
	month *string
	year  *int
}

func (f *fakeStatsSvc) Get(_ context.Context, q stats.GetStatsQuery) (stats.AdminStats, error) {
	f.calls++
	f.month, f.year = q.Month, q.Year
	return stats.AdminStats{Month: "September", Year: 2026, ReportsTotal: 3, ReportsApproved: 1, ReportsPending: 2, ReportsNeedsRevision: 0, UsersTotal: 7, AdminsTotal: 2, ExpectedReports: 5, NoReport: 2, ChurchesTotal: 4}, f.err
}

func (f *fakeStatsSvc) Tracker(_ context.Context, q stats.GetStatsQuery) (stats.Tracker, error) {
	f.calls++
	f.month, f.year = q.Month, q.Year
	rows := []stats.TrackerRow{{UserID: 4, Name: "Ana", Email: "ana@example.com", ChurchName: "Bethel", UserStatus: "active", SubmissionStatus: stats.TrackerSubmitted, ReviewStatus: stats.TrackerApproved}}
	return stats.Tracker{Month: "September", Year: 2026, Rows: rows}, f.err
}

func (f *fakeStatsSvc) AnalyticsAttendance(_ context.Context, q stats.AttendanceQuery) (stats.AttendanceAnalytics, error) {
	f.calls++
	return stats.AttendanceAnalytics{
		Year: q.Year, Metric: q.Metric, Status: q.Status,
		Trend:    []stats.AttendanceTrendPoint{{Month: "January", Week: 1, Value: 12.5}},
		Churches: []stats.AttendanceChurch{{ChurchName: "Bethel", Value: 12.5}},
	}, f.err
}

func serveStats(t *testing.T, svc stats.Service, target string) *httptest.ResponseRecorder {
	t.Helper()
	e := echo.New()
	e.HTTPErrorHandler = apperror.ErrorHandler
	h := NewStatsHandler(svc)
	e.GET("/admin/stats", h.GetStats)
	e.GET("/admin/tracker", h.GetTracker)
	e.GET("/admin/analytics/attendance", h.GetAttendanceAnalytics)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, httptest.NewRequestWithContext(context.Background(), http.MethodGet, target, nil))
	return rec
}

func TestStatsHandler_GetStats(t *testing.T) {
	t.Run("answers the counts as snake_case JSON", func(t *testing.T) {
		rec := serveStats(t, &fakeStatsSvc{}, "/admin/stats")
		if rec.Code != http.StatusOK {
			t.Fatalf("status = %d, body %s", rec.Code, rec.Body)
		}
		var got map[string]any
		if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
			t.Fatal(err)
		}
		want := map[string]any{
			"month": "September", "year": 2026.0, "reports_total": 3.0, "reports_approved": 1.0,
			"reports_pending": 2.0, "users_total": 7.0, "admins_total": 2.0,
			"reports_needs_revision": 0.0,
			"expected_reports":       5.0, "no_report": 2.0, "churches_total": 4.0,
		}
		if fmt.Sprint(got) != fmt.Sprint(want) {
			t.Errorf("body = %v, want %v", got, want)
		}
	})

	t.Run("no query means no period is passed on", func(t *testing.T) {
		svc := &fakeStatsSvc{}
		serveStats(t, svc, "/admin/stats")
		if svc.month != nil || svc.year != nil {
			t.Errorf("period = %v %v, want both nil", svc.month, svc.year)
		}
	})

	t.Run("month and year are passed on", func(t *testing.T) {
		svc := &fakeStatsSvc{}
		serveStats(t, svc, "/admin/stats?month=march&year=2024")
		if svc.month == nil || *svc.month != "march" || svc.year == nil || *svc.year != 2024 {
			t.Errorf("period = %v %v", svc.month, svc.year)
		}
	})

	t.Run("an empty month is the same as none", func(t *testing.T) {
		svc := &fakeStatsSvc{}
		serveStats(t, svc, "/admin/stats?month=&year=")
		if svc.month != nil || svc.year != nil {
			t.Errorf("period = %v %v, want both nil", svc.month, svc.year)
		}
	})

	for _, year := range []string{"abc", "0", "-3", "20.5"} {
		t.Run("a bad year is 400: "+year, func(t *testing.T) {
			svc := &fakeStatsSvc{}
			rec := serveStats(t, svc, "/admin/stats?month=March&year="+year)
			if rec.Code != http.StatusBadRequest {
				t.Errorf("status = %d, want 400", rec.Code)
			}
			if svc.calls != 0 {
				t.Error("the service was called with a bad year")
			}
		})
	}

	t.Run("a period the service rejects is 422", func(t *testing.T) {
		rec := serveStats(t, &fakeStatsSvc{err: fmt.Errorf("x: %w", stats.ErrPeriod)}, "/admin/stats?month=March")
		if rec.Code != http.StatusUnprocessableEntity {
			t.Errorf("status = %d, want 422", rec.Code)
		}
		var body errorBody
		if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil || body.Error != "VALIDATION_ERROR" {
			t.Errorf("body = %s (%v)", rec.Body, err)
		}
	})

	t.Run("any other error is a 500", func(t *testing.T) {
		rec := serveStats(t, &fakeStatsSvc{err: errors.New("connection refused to 10.0.0.5")}, "/admin/stats")
		if rec.Code != http.StatusInternalServerError {
			t.Errorf("status = %d, want 500", rec.Code)
		}
	})
}

// The period parsing is shared with GetStats and covered there; these check the wiring.
func TestStatsHandler_GetTracker(t *testing.T) {
	t.Run("answers the rows as snake_case JSON", func(t *testing.T) {
		svc := &fakeStatsSvc{}
		rec := serveStats(t, svc, "/admin/tracker?month=september&year=2026")
		if rec.Code != http.StatusOK {
			t.Fatalf("status = %d, body %s", rec.Code, rec.Body)
		}
		if svc.month == nil || *svc.month != "september" || svc.year == nil || *svc.year != 2026 {
			t.Errorf("period = %v %v", svc.month, svc.year)
		}
		var got map[string]any
		if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
			t.Fatal(err)
		}
		want := map[string]any{
			"month": "September", "year": 2026.0,
			"rows": []any{map[string]any{
				"user_id": 4.0, "name": "Ana", "email": "ana@example.com", "church_name": "Bethel",
				"user_status": "active", "submission_status": "submitted", "review_status": "approved",
			}},
		}
		if fmt.Sprint(got) != fmt.Sprint(want) {
			t.Errorf("body = %v, want %v", got, want)
		}
	})

	t.Run("a bad year is 400", func(t *testing.T) {
		svc := &fakeStatsSvc{}
		if rec := serveStats(t, svc, "/admin/tracker?month=March&year=abc"); rec.Code != http.StatusBadRequest {
			t.Errorf("status = %d, want 400", rec.Code)
		}
		if svc.calls != 0 {
			t.Error("the service was called with a bad year")
		}
	})

	t.Run("a month without a year is 422 on month", func(t *testing.T) {
		rec := serveStats(t, &fakeStatsSvc{err: stats.ErrPeriod}, "/admin/tracker?month=March")
		if rec.Code != http.StatusUnprocessableEntity {
			t.Errorf("status = %d, want 422", rec.Code)
		}
		var body struct {
			Error   string            `json:"error"`
			Details map[string]string `json:"details"`
		}
		if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil || body.Error != "VALIDATION_ERROR" || body.Details["month"] == "" {
			t.Errorf("body = %s (%v)", rec.Body, err)
		}
	})

	t.Run("any other error is a 500", func(t *testing.T) {
		if rec := serveStats(t, &fakeStatsSvc{err: errors.New("boom")}, "/admin/tracker"); rec.Code != http.StatusInternalServerError {
			t.Errorf("status = %d, want 500", rec.Code)
		}
	})
}

func TestStatsHandler_GetAnalyticsAttendance(t *testing.T) {
	svc := &fakeStatsSvc{}
	rec := serveStats(t, svc, "/admin/analytics/attendance?year=2026&metric=worship_service&status=all")
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body %s", rec.Code, rec.Body)
	}
	var got map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
		t.Fatal(err)
	}
	if got["year"] != 2026.0 || got["metric"] != "worship_service" || got["status"] != "all" {
		t.Fatalf("body = %v", got)
	}

	t.Run("default status is approved", func(t *testing.T) {
		svc := &fakeStatsSvc{}
		serveStats(t, svc, "/admin/analytics/attendance?year=2026&metric=worship_service")
		if svc.calls != 1 {
			t.Fatalf("calls = %d", svc.calls)
		}
	})

	for _, target := range []string{
		"/admin/analytics/attendance?metric=worship_service",
		"/admin/analytics/attendance?year=2026",
		"/admin/analytics/attendance?year=nope&metric=worship_service",
	} {
		t.Run(target, func(t *testing.T) {
			svc := &fakeStatsSvc{}
			if rec := serveStats(t, svc, target); rec.Code != http.StatusBadRequest {
				t.Errorf("status = %d, want 400", rec.Code)
			}
			if svc.calls != 0 {
				t.Error("service was called")
			}
		})
	}
}
