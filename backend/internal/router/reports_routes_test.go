package router

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"

	"github.com/labstack/echo/v4"
	"golang.org/x/time/rate"

	"github.com/ssr0016/pastoral-reports/backend/internal/accesscontrol"
	"github.com/ssr0016/pastoral-reports/backend/internal/apperror"
	authrest "github.com/ssr0016/pastoral-reports/backend/internal/auth/rest"
	ourmiddleware "github.com/ssr0016/pastoral-reports/backend/internal/middleware"
	"github.com/ssr0016/pastoral-reports/backend/internal/report"
	reportrest "github.com/ssr0016/pastoral-reports/backend/internal/report/rest"
	"github.com/ssr0016/pastoral-reports/backend/internal/request"
	"github.com/ssr0016/pastoral-reports/backend/internal/user"
	"github.com/ssr0016/pastoral-reports/backend/internal/user/userimpl"
	"github.com/ssr0016/pastoral-reports/backend/internal/validator"
	"github.com/ssr0016/pastoral-reports/backend/pkg/pagination"
)

// permRoles serves a fixed permission list for every role.
type permRoles struct {
	accesscontrol.RoleStore
	perms []string
}

func (p permRoles) GetPermissions(context.Context, int64) ([]accesscontrol.Permission, error) {
	var out []accesscontrol.Permission
	for _, n := range p.perms {
		out = append(out, accesscontrol.Permission{Name: n})
	}
	return out, nil
}

// okReportSvc answers every call successfully; it proves a request got past the gate.
type okReportSvc struct{}

func (okReportSvc) Create(context.Context, int64, string, report.CreateReportRequest) (*report.ReportResponse, error) {
	return &report.ReportResponse{}, nil
}
func (okReportSvc) Get(context.Context, int64, int64) (*report.ReportResponse, error) {
	return &report.ReportResponse{}, nil
}
func (okReportSvc) List(context.Context, int64, report.ListReportsParams, pagination.Params) ([]report.ReportResponse, int64, error) {
	return nil, 0, nil
}
func (okReportSvc) Update(context.Context, int64, string, int64, report.UpdateReportRequest) (*report.ReportResponse, error) {
	return &report.ReportResponse{}, nil
}
func (okReportSvc) Delete(context.Context, int64, string, int64) error { return nil }
func (okReportSvc) ToggleComplete(context.Context, int64, string, int64, ...string) (*report.ReportMetadata, error) {
	return &report.ReportMetadata{}, nil
}
func (okReportSvc) RequestRevision(context.Context, int64, string, int64, ...string) (*report.ReportMetadata, error) {
	return &report.ReportMetadata{}, nil
}
func (okReportSvc) ExportOne(context.Context, int64, string, int64) (*report.Export, error) {
	return &report.Export{Filename: "r.xlsx"}, nil
}
func (okReportSvc) ExportMonth(context.Context, int64, string, report.ExportReportsRequest) (*report.Export, error) {
	return &report.Export{Filename: "r.xlsx"}, nil
}
func (okReportSvc) Preview(context.Context, int64, report.ExcelPreviewRequest) (*report.ExcelPreview, error) {
	return &report.ExcelPreview{Count: 1}, nil
}

// reportsServer wires the real routes. A request is logged in as user 1 when it
// carries the X-Test-User header; that user's role holds exactly perms.
func reportsServer(perms ...string) *echo.Echo {
	return reportsServerLimited(func() echo.MiddlewareFunc { return noLimit }, perms...)
}

func reportsServerLimited(mkLimit func() echo.MiddlewareFunc, perms ...string) *echo.Echo {
	return reportsServerWith(func() Limiters {
		return Limiters{Global: noLimit, Read: noLimit, Write: mkLimit()}
	}, perms...)
}

func reportsServerWith(mk func() Limiters, perms ...string) *echo.Echo {
	return reportsServerWithRole(mk, "user", perms...)
}

func reportsServerWithRole(mk func() Limiters, role string, perms ...string) *echo.Echo {
	limits := mk()
	users := userimpl.NewMockStore()
	users.GetWithRoleFunc = func(_ context.Context, id int64) (*user.User, error) {
		return &user.User{ID: id, RoleID: 9, Role: &accesscontrol.Role{Name: role}}, nil
	}

	e := echo.New()
	e.Validator = validator.New()
	e.HTTPErrorHandler = apperror.ErrorHandler
	e.Use(func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			if id, err := strconv.ParseInt(c.Request().Header.Get("X-Test-User"), 10, 64); err == nil {
				c.SetRequest(c.Request().WithContext(request.WithUser(c.Request().Context(), request.User{ID: id})))
			}
			return next(c)
		}
	})
	users.Roles = permRoles{perms: perms}
	Setup(e, noAuthn, users,
		authrest.NewAuthHandler(nil, nil), nil, nil, nil, nil, nil,
		authrest.NewVerificationHandler(nil), authrest.NewPasswordResetHandler(nil),
		reportrest.NewReportHandler(okReportSvc{}), nil, nil, limits,
	)
	return e
}

func reportRequest(e *echo.Echo, method, path, body string, loggedIn bool) int {
	return reportRequestAs(e, method, path, body, map[bool]string{true: "1", false: ""}[loggedIn])
}

// reportRequestAs sends the request as the given user ID ("" = no session).
func reportRequestAs(e *echo.Echo, method, path, body, user string) int {
	req := httptest.NewRequestWithContext(context.Background(), method, path, strings.NewReader(body))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	if user != "" {
		req.Header.Set("X-Test-User", user)
	}
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)
	return rec.Code
}

var reportRoutes = []struct {
	name, method, path, body, perm string
}{
	{"list", http.MethodGet, "/api/v1/reports", "", "reports:read"},
	{"create", http.MethodPost, "/api/v1/reports", `{"month":"May"}`, "reports:write"},
	{"get", http.MethodGet, "/api/v1/reports/1", "", "reports:read"},
	{"update", http.MethodPut, "/api/v1/reports/1", `{"month":"May"}`, "reports:write"},
	{"delete", http.MethodDelete, "/api/v1/reports/1", "", "reports:delete"},
	{"toggle", http.MethodPatch, "/api/v1/reports/1/complete", "", "reports:approve"},
}

func TestReportsRoutes_NoSessionIs401(t *testing.T) {
	e := reportsServer("reports:read", "reports:write", "reports:delete", "reports:approve")
	for _, rt := range reportRoutes {
		t.Run(rt.name, func(t *testing.T) {
			if got := reportRequest(e, rt.method, rt.path, rt.body, false); got != http.StatusUnauthorized {
				t.Errorf("status = %d, want 401", got)
			}
		})
	}
}

func TestReportsRoutes_MissingPermissionIs403(t *testing.T) {
	// Holds every reports permission except the one under test.
	all := []string{"reports:read", "reports:write", "reports:delete", "reports:approve", "reports:read_any", "reports:write_any"}
	for _, rt := range reportRoutes {
		t.Run(rt.name, func(t *testing.T) {
			var others []string
			for _, p := range all {
				if p != rt.perm {
					others = append(others, p)
				}
			}
			e := reportsServer(others...)
			if got := reportRequest(e, rt.method, rt.path, rt.body, true); got != http.StatusForbidden {
				t.Errorf("status = %d, want 403", got)
			}
		})
	}
}

func TestReportsRoutes_WithPermissionReachesHandler(t *testing.T) {
	for _, rt := range reportRoutes {
		t.Run(rt.name, func(t *testing.T) {
			e := reportsServer(rt.perm)
			got := reportRequest(e, rt.method, rt.path, rt.body, true)
			if got < 200 || got >= 300 {
				t.Errorf("status = %d, want 2xx", got)
			}
		})
	}
}

func TestReportsRoutes_CreateRequiresUserRole(t *testing.T) {
	for _, role := range []string{"user", "editor", "admin", "manager", "custom", ""} {
		t.Run(role, func(t *testing.T) {
			e := reportsServerWithRole(func() Limiters {
				return Limiters{Global: noLimit, Read: noLimit, Write: noLimit}
			}, role, "reports:write", "reports:write_any")
			want := http.StatusForbidden
			if role == "user" {
				want = http.StatusCreated
			}
			if got := reportRequest(e, http.MethodPost, "/api/v1/reports", `{"month":"May"}`, true); got != want {
				t.Errorf("status = %d, want %d", got, want)
			}
		})
	}
}

// All four write routes share one bucket; reads are never limited.
func TestReportsRoutes_WriteLimitSharedAndReadsFree(t *testing.T) {
	// burst 3, practically no refill during the test
	e := reportsServerLimited(userLimit(3),
		"reports:read", "reports:write", "reports:delete", "reports:approve")

	writes := []struct{ method, path, body string }{
		{http.MethodPost, "/api/v1/reports", `{"month":"May"}`},
		{http.MethodPut, "/api/v1/reports/1", `{"month":"May"}`},
		{http.MethodDelete, "/api/v1/reports/1", ""},
	}
	for _, w := range writes {
		if got := reportRequest(e, w.method, w.path, w.body, true); got != http.StatusOK && got != http.StatusCreated && got != http.StatusNoContent {
			t.Fatalf("%s %s within burst: status = %d", w.method, w.path, got)
		}
	}
	// the fourth write, on a different route, hits the shared bucket
	if got := reportRequest(e, http.MethodPatch, "/api/v1/reports/1/complete", "", true); got != http.StatusTooManyRequests {
		t.Errorf("PATCH after burst: status = %d, want 429", got)
	}
	// reads are not limited
	for i := 0; i < 10; i++ {
		if got := reportRequest(e, http.MethodGet, "/api/v1/reports", "", true); got != http.StatusOK {
			t.Fatalf("GET #%d: status = %d, want 200", i, got)
		}
	}
}

// A user without the write permission must not use tokens: otherwise they could
// lock out everyone sharing the bucket.
func TestReportsRoutes_DeniedRequestsDoNotConsumeTokens(t *testing.T) {
	e := reportsServerLimited(userLimit(2), "reports:read")
	for i := 0; i < 10; i++ {
		if got := reportRequest(e, http.MethodPost, "/api/v1/reports", `{"month":"May"}`, true); got != http.StatusForbidden {
			t.Fatalf("POST #%d: status = %d, want 403 (never 429)", i, got)
		}
	}
}

// userLimit builds a per-user limiter with the given burst and practically no refill.
func userLimit(burst int) func() echo.MiddlewareFunc {
	return func() echo.MiddlewareFunc {
		return ourmiddleware.RateLimitByUser(rate.Limit(0.0001), burst)
	}
}

// Two users on the same IP (every test request comes from the same address) get
// separate buckets: one exhausting theirs does not block the other.
func TestReportsRoutes_WriteLimitIsPerUser(t *testing.T) {
	e := reportsServerLimited(userLimit(2), "reports:write")
	post := func(user string) int {
		return reportRequestAs(e, http.MethodPost, "/api/v1/reports", `{"month":"May"}`, user)
	}
	for i := 0; i < 2; i++ {
		if got := post("1"); got != http.StatusCreated {
			t.Fatalf("user 1 POST #%d: status = %d, want 201", i, got)
		}
	}
	if got := post("1"); got != http.StatusTooManyRequests {
		t.Errorf("user 1 over burst: status = %d, want 429", got)
	}
	if got := post("2"); got != http.StatusCreated {
		t.Errorf("user 2 on the same IP: status = %d, want 201", got)
	}
}
