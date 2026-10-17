package handler

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/alexedwards/scs/v2"
	"github.com/labstack/echo/v4"
	"golang.org/x/crypto/bcrypt"

	"github.com/ssr0016/pastoral-reports/backend/internal/apperror"
	"github.com/ssr0016/pastoral-reports/backend/internal/model"
	"github.com/ssr0016/pastoral-reports/backend/internal/repository"
	"github.com/ssr0016/pastoral-reports/backend/internal/service"
	"github.com/ssr0016/pastoral-reports/backend/internal/validator"
)

const loginPassword = "X7kP9mQ2vL8nR4tY6wB3zC5dF1gH0jK"

// serveLogin signs in as a stored account with the given status and lock time.
func serveLogin(t *testing.T, status string, lockedUntil *time.Time, password string) *httptest.ResponseRecorder {
	t.Helper()
	hash, err := bcrypt.GenerateFromPassword([]byte(loginPassword), bcrypt.MinCost)
	if err != nil {
		t.Fatal(err)
	}
	users := repository.NewMockUserRepo()
	u, err := users.CreateWithPassword(context.Background(), "s@example.com", "S", string(hash))
	if err != nil {
		t.Fatal(err)
	}
	u.Status, u.LockedUntil = status, lockedUntil

	sm := scs.New()
	h := NewAuthHandler(service.NewAuthService(users, nil, nil, nil, nil, nil), sm)
	e := echo.New()
	e.Validator = validator.New()
	e.HTTPErrorHandler = apperror.ErrorHandler
	e.Use(echo.WrapMiddleware(sm.LoadAndSave))
	e.POST("/auth/login", h.Login)

	body := `{"email":"s@example.com","password":"` + password + `"}`
	req := httptest.NewRequestWithContext(context.Background(), http.MethodPost, "/auth/login", strings.NewReader(body))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)
	return rec
}

func TestAuthHandler_Login_StatusMessages(t *testing.T) {
	future := time.Now().Add(time.Hour)
	tests := []struct {
		name     string
		status   string
		locked   *time.Time
		password string
		code     int
		message  string
	}{
		{"active signs in", model.UserStatusActive, nil, loginPassword, http.StatusOK, ""},
		{"inactive", model.UserStatusInactive, nil, loginPassword, http.StatusForbidden, "Your account is inactive. Contact your admin."},
		{"disciplinary", model.UserStatusDisciplinary, nil, loginPassword, http.StatusForbidden, "Your account is under review. Contact your admin."},
		{"deceased", model.UserStatusDeceased, nil, loginPassword, http.StatusForbidden, "This account is closed."},
		{"wrong password on a closed account stays generic", model.UserStatusDeceased, nil, "wrong-password", http.StatusUnauthorized, "invalid email or password"},
		{"locked", model.UserStatusActive, &future, loginPassword, http.StatusForbidden, "Account locked. Try again later."},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rec := serveLogin(t, tt.status, tt.locked, tt.password)
			if rec.Code != tt.code {
				t.Fatalf("status = %d, want %d (%s)", rec.Code, tt.code, rec.Body.String())
			}
			if tt.message != "" && !strings.Contains(rec.Body.String(), tt.message) {
				t.Errorf("body = %s, want it to contain %q", rec.Body.String(), tt.message)
			}
		})
	}
}
