package router

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"

	"github.com/alexedwards/scs/v2"
	"github.com/labstack/echo/v4"
	"golang.org/x/time/rate"

	"github.com/ssr0016/pastoral-reports/backend/internal/apperror"
	"github.com/ssr0016/pastoral-reports/backend/internal/handler"
	ourmiddleware "github.com/ssr0016/pastoral-reports/backend/internal/middleware"
	"github.com/ssr0016/pastoral-reports/backend/internal/model"
	"github.com/ssr0016/pastoral-reports/backend/internal/repository"
	"github.com/ssr0016/pastoral-reports/backend/internal/validator"
	"github.com/ssr0016/pastoral-reports/backend/pkg/pagination"
)

// permRoles serves a fixed permission list for every role.
type permRoles struct {
	repository.RoleRepository
	perms []string
}

func (p permRoles) GetPermissions(context.Context, int64) ([]model.Permission, error) {
	var out []model.Permission
	for _, n := range p.perms {
		out = append(out, model.Permission{Name: n})
	}
	return out, nil
}

// okReportSvc answers every call successfully; it proves a request got past the gate.
type okReportSvc struct{}

func (okReportSvc) Create(context.Context, int64, string, model.CreateReportRequest) (*model.ReportResponse, error) {
	return &model.ReportResponse{}, nil
}
func (okReportSvc) Get(context.Context, int64, int64) (*model.ReportResponse, error) {
	return &model.ReportResponse{}, nil
}
func (okReportSvc) List(context.Context, int64, model.ListReportsParams, pagination.Params) ([]model.ReportResponse, int64, error) {
	return nil, 0, nil
}
func (okReportSvc) Update(context.Context, int64, string, int64, model.UpdateReportRequest) (*model.ReportResponse, error) {
	return &model.ReportResponse{}, nil
}
func (okReportSvc) Delete(context.Context, int64, string, int64) error { return nil }
func (okReportSvc) ToggleComplete(context.Context, int64, string, int64) (*model.ReportMetadata, error) {
	return &model.ReportMetadata{}, nil
}

// reportsServer wires the real routes. A request is logged in as user 1 when it
// carries the X-Test-User header; that user's role holds exactly perms.
func reportsServer(perms ...string) *echo.Echo {
	return reportsServerLimited(func(*scs.SessionManager) echo.MiddlewareFunc { return noLimit }, perms...)
}

func reportsServerLimited(mkLimit func(*scs.SessionManager) echo.MiddlewareFunc, perms ...string) *echo.Echo {
	return reportsServerWith(func(sm *scs.SessionManager) Limiters {
		return Limiters{Global: noLimit, Read: noLimit, Write: mkLimit(sm)}
	}, perms...)
}

func reportsServerWith(mk func(*scs.SessionManager) Limiters, perms ...string) *echo.Echo {
	sm := scs.New()
	limits := mk(sm)
	users := repository.NewMockUserRepo()
	users.GetWithRoleFunc = func(_ context.Context, id int64) (*model.User, error) {
		return &model.User{ID: id, RoleID: 9}, nil
	}

	e := echo.New()
	e.Validator = validator.New()
	e.HTTPErrorHandler = apperror.ErrorHandler
	e.Use(echo.WrapMiddleware(sm.LoadAndSave))
	e.Use(func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			if id, err := strconv.ParseInt(c.Request().Header.Get("X-Test-User"), 10, 64); err == nil {
				sm.Put(c.Request().Context(), "user_id", id)
			}
			return next(c)
		}
	})
	Setup(e, sm, users, permRoles{perms: perms},
		handler.NewAuthHandler(nil, nil), nil, nil, nil, nil,
		handler.NewVerificationHandler(nil), handler.NewPasswordResetHandler(nil),
		handler.NewReportHandler(okReportSvc{}, sm), nil, limits,
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
func userLimit(burst int) func(*scs.SessionManager) echo.MiddlewareFunc {
	return func(sm *scs.SessionManager) echo.MiddlewareFunc {
		return ourmiddleware.RateLimitByUser(sm, rate.Limit(0.0001), burst)
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
