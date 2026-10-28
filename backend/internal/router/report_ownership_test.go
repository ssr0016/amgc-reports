package router

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/labstack/echo/v4"

	"github.com/ssr0016/pastoral-reports/backend/internal/accesscontrol"
	"github.com/ssr0016/pastoral-reports/backend/internal/apperror"
	"github.com/ssr0016/pastoral-reports/backend/internal/audit/auditimpl"
	authrest "github.com/ssr0016/pastoral-reports/backend/internal/auth/rest"
	"github.com/ssr0016/pastoral-reports/backend/internal/report"
	"github.com/ssr0016/pastoral-reports/backend/internal/report/reportimpl"
	reportrest "github.com/ssr0016/pastoral-reports/backend/internal/report/rest"
	"github.com/ssr0016/pastoral-reports/backend/internal/request"
	"github.com/ssr0016/pastoral-reports/backend/internal/user"
	"github.com/ssr0016/pastoral-reports/backend/internal/user/userimpl"
	"github.com/ssr0016/pastoral-reports/backend/internal/validator"
)

const (
	ownerUser  = "1" // plain user who files the report
	otherUser  = "2" // plain user, not the owner
	editorUser = "3" // holds the _any permissions
	adminUser  = "4" // holds every permission
	readerUser = "5" // a plain user as seeded: reads every report, changes only their own
)

// rolePerms maps a user ID to the role ID and permissions the fixtures give it.
var rolePerms = map[int64][]string{
	1: {"reports:read", "reports:write", "reports:delete"},
	2: {"reports:read", "reports:write", "reports:delete"},
	3: {"reports:read", "reports:write", "reports:delete", "reports:read_any", "reports:write_any", "reports:delete_any"},
	4: {"reports:read", "reports:write", "reports:delete", "reports:read_any", "reports:write_any", "reports:delete_any", "reports:approve"},
	5: {"reports:read", "reports:write", "reports:delete", "reports:read_any"},
}

// byRole serves permissions by role ID; a user's role ID is their user ID here.
type byRole struct{ accesscontrol.RoleStore }

func (byRole) GetPermissions(_ context.Context, roleID int64) ([]accesscontrol.Permission, error) {
	var out []accesscontrol.Permission
	for _, n := range rolePerms[roleID] {
		out = append(out, accesscontrol.Permission{Name: n})
	}
	return out, nil
}

// runTx runs fn directly: the in-memory mocks have no transaction to roll back.
type runTx struct{}

func (runTx) InTransaction(ctx context.Context, fn func(context.Context) error) error { return fn(ctx) }

// previousMonthBody is a create body for the month a plain user may file, so the
// seed POST passes the previous-month rule whatever day the tests run (D-F8).
func previousMonthBody() string { return `{"month":"` + previousMonthName() + `"}` }

func previousMonthName() string {
	loc, err := time.LoadLocation("Asia/Manila")
	if err != nil {
		panic(err)
	}
	t := time.Now().In(loc)
	prev := time.Date(t.Year(), t.Month(), 1, 0, 0, 0, 0, loc).AddDate(0, -1, 0)
	return report.Months[prev.Month()-1]
}

// ownershipServer wires the real routes, handler and report service over in-memory
// repositories, so the ownership rule is exercised through HTTP. Unlike
// reportsServer it has no stub service.
func ownershipServer() *echo.Echo {
	users := userimpl.NewMockStore()
	users.GetWithRoleFunc = func(_ context.Context, id int64) (*user.User, error) {
		role := "user"
		switch id {
		case 3:
			role = "editor"
		case 4:
			role = "admin"
		}
		return &user.User{ID: id, RoleID: id, Role: &accesscontrol.Role{Name: role}}, nil
	}
	users.Roles = byRole{}
	svc := reportimpl.NewService(reportimpl.NewMockStore(), auditimpl.NewMockStore(), runTx{}, users)

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
	Setup(e, noAuthn, users,
		authrest.NewAuthHandler(nil, nil), nil, nil, nil, nil, nil,
		authrest.NewVerificationHandler(nil), authrest.NewPasswordResetHandler(nil),
		reportrest.NewReportHandler(svc), nil, nil,
		Limiters{Global: noLimit, Read: noLimit, Write: noLimit, Export: noLimit},
	)
	return e
}

// Another user's report is a 404, never a 403, so its existence is not disclosed (D-1).
// The _any permissions (editor, admin) reach it; the owner always can. A user with
// only read_any (the seeded user role) can read it but not change or approve it.
func TestReportsOwnership(t *testing.T) {
	const report = "/api/v1/reports/1"
	body := previousMonthBody()
	cases := []struct {
		name, user, method, body string
		want                     int
	}{
		{"non-owner PUT", otherUser, http.MethodPut, body, http.StatusNotFound},
		{"non-owner DELETE", otherUser, http.MethodDelete, "", http.StatusNotFound},
		{"non-owner GET", otherUser, http.MethodGet, "", http.StatusNotFound},
		{"editor PUT", editorUser, http.MethodPut, body, http.StatusOK},
		{"editor DELETE", editorUser, http.MethodDelete, "", http.StatusNoContent},
		{"admin PUT", adminUser, http.MethodPut, body, http.StatusOK},
		{"owner PUT", ownerUser, http.MethodPut, body, http.StatusOK},
		{"owner DELETE", ownerUser, http.MethodDelete, "", http.StatusNoContent},
		{"reader GET", readerUser, http.MethodGet, "", http.StatusOK},
		{"reader PUT", readerUser, http.MethodPut, body, http.StatusNotFound},
		{"reader DELETE", readerUser, http.MethodDelete, "", http.StatusNotFound},
		{"reader approve", readerUser, http.MethodPatch, "", http.StatusForbidden},
		{"editor approve", editorUser, http.MethodPatch, "", http.StatusForbidden},
		{"admin approve", adminUser, http.MethodPatch, "", http.StatusOK},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			e := ownershipServer()
			if got := reportRequestAs(e, http.MethodPost, "/api/v1/reports", body, ownerUser); got != http.StatusCreated {
				t.Fatalf("seed POST: status = %d, want 201", got)
			}
			path := report
			if tc.method == http.MethodPatch {
				path += "/complete" // the only PATCH route is approve
			}
			if got := reportRequestAs(e, tc.method, path, tc.body, tc.user); got != tc.want {
				t.Errorf("status = %d, want %d", got, tc.want)
			}
		})
	}
}

// With read_any the list holds every user's reports; without it, only the caller's own.
func TestReportsList_ReadAny(t *testing.T) {
	cases := []struct {
		name, user string
		want       int
	}{
		{"reader sees the owner's report", readerUser, 1},
		{"user without read_any does not", otherUser, 0},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			e := ownershipServer()
			if got := reportRequestAs(e, http.MethodPost, "/api/v1/reports", previousMonthBody(), ownerUser); got != http.StatusCreated {
				t.Fatalf("seed POST: status = %d, want 201", got)
			}
			req := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/api/v1/reports", strings.NewReader(""))
			req.Header.Set("X-Test-User", tc.user)
			rec := httptest.NewRecorder()
			e.ServeHTTP(rec, req)
			var page struct {
				Data []json.RawMessage `json:"data"`
			}
			if err := json.Unmarshal(rec.Body.Bytes(), &page); err != nil || rec.Code != http.StatusOK {
				t.Fatalf("status = %d, body = %s", rec.Code, rec.Body)
			}
			if len(page.Data) != tc.want {
				t.Errorf("list has %d reports, want %d", len(page.Data), tc.want)
			}
		})
	}
}

// D-18: a reader (read_any only) gets the report without free text or tithes_offering;
// the owner and an editor get everything.
func TestReportsVisibility(t *testing.T) {
	body := `{"month":"` + previousMonthName() + `","narrative_report":"secret","prayer_request":"pray","metrics":{"tithes_offering":{"week1":5},"outreach":{"week1":2}}}`
	cases := []struct {
		name, user string
		full       bool
	}{
		{"owner", ownerUser, true},
		{"editor", editorUser, true},
		{"reader", readerUser, false},
	}
	for _, path := range []string{"/api/v1/reports/1", "/api/v1/reports"} {
		for _, tc := range cases {
			t.Run(tc.name+" "+path, func(t *testing.T) {
				e := ownershipServer()
				if got := reportRequestAs(e, http.MethodPost, "/api/v1/reports", body, ownerUser); got != http.StatusCreated {
					t.Fatalf("seed POST: status = %d, want 201", got)
				}
				req := httptest.NewRequestWithContext(context.Background(), http.MethodGet, path, strings.NewReader(""))
				req.Header.Set("X-Test-User", tc.user)
				rec := httptest.NewRecorder()
				e.ServeHTTP(rec, req)
				if rec.Code != http.StatusOK {
					t.Fatalf("status = %d, body = %s", rec.Code, rec.Body)
				}
				out := rec.Body.String()
				for _, hidden := range []string{"secret", "pray", "tithes_offering"} {
					if got := strings.Contains(out, hidden); got != tc.full {
						t.Errorf("body contains %q = %v, want %v", hidden, got, tc.full)
					}
				}
				if !strings.Contains(out, "outreach") {
					t.Errorf("other metrics must stay visible: %s", out)
				}
			})
		}
	}
}
