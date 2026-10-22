package authimpl

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ssr0016/pastoral-reports/backend/internal/audit"
	"github.com/ssr0016/pastoral-reports/backend/internal/audit/auditimpl"
	"github.com/ssr0016/pastoral-reports/backend/internal/auth"
	"github.com/ssr0016/pastoral-reports/backend/internal/user"
	"github.com/ssr0016/pastoral-reports/backend/internal/user/userimpl"
)

// Every refused sign-in writes one LOGIN_FAILED row (D-A4, amended 2026-10-22).
// Nobody signed in, so the row has no actor; an unknown or deleted email has no
// target either, and the typed email is never stored.
func TestLogin_AuditsEachRefusal(t *testing.T) {
	future := time.Now().Add(time.Hour)
	tests := []struct {
		name       string
		email      string
		password   string
		setup      func(u *user.User)
		wantErr    error
		wantReason string
		wantTarget bool
	}{
		{"unknown email", "nobody@example.com", testPassword, nil, auth.ErrInvalidCredentials, audit.ReasonUnknownAccount, false},
		{"deleted user", "a@example.com", testPassword, func(u *user.User) { now := time.Now(); u.DeletedAt = &now }, auth.ErrInvalidCredentials, audit.ReasonUnknownAccount, false},
		{"wrong password", "a@example.com", "wrong-password", nil, auth.ErrInvalidCredentials, audit.ReasonWrongPassword, true},
		{"locked", "a@example.com", testPassword, func(u *user.User) { u.LockedUntil = &future }, auth.ErrAccountLocked, audit.ReasonLocked, true},
		{"inactive", "a@example.com", testPassword, func(u *user.User) { u.Status = user.UserStatusInactive }, auth.ErrAccountInactive, user.UserStatusInactive, true},
		{"disciplinary", "a@example.com", testPassword, func(u *user.User) { u.Status = user.UserStatusDisciplinary }, auth.ErrAccountUnderReview, user.UserStatusDisciplinary, true},
		{"deceased", "a@example.com", testPassword, func(u *user.User) { u.Status = user.UserStatusDeceased }, auth.ErrAccountClosed, user.UserStatusDeceased, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repo := userimpl.NewMockStore()
			log := auditimpl.NewMockStore()
			svc := newService(repo, nil, nil, log, nil, discardLog)
			u := seedUser(t, repo, "a@example.com", "Ana")
			if tt.setup != nil {
				tt.setup(u)
			}

			_, err := svc.Login(context.Background(), tt.email, tt.password, "10.0.0.9")
			require.ErrorIs(t, err, tt.wantErr)

			rows := log.Entries()
			require.Len(t, rows, 1)
			e := rows[0]
			assert.Equal(t, audit.ActionLoginFailed, e.Action)
			assert.Equal(t, audit.TargetTypeUser, e.TargetType)
			assert.Equal(t, map[string]any{"reason": tt.wantReason}, e.Details)
			assert.Nil(t, e.PerformedBy)
			assert.Nil(t, e.PerformedByName)
			if tt.wantTarget {
				require.NotNil(t, e.TargetID)
				assert.Equal(t, strconv.FormatInt(u.ID, 10), *e.TargetID)
				assert.Equal(t, u.Email, *e.TargetName)
			} else {
				assert.Nil(t, e.TargetID)
				assert.Nil(t, e.TargetName)
				assert.NotContains(t, fmt.Sprintf("%+v", e), tt.email, "the typed email must not be stored")
			}
		})
	}
}

func TestLogin_SuccessWritesNoFailedRow(t *testing.T) {
	repo := userimpl.NewMockStore()
	log := auditimpl.NewMockStore()
	svc := newService(repo, nil, nil, log, nil, discardLog)
	seedUser(t, repo, "a@example.com", "Ana")

	_, err := svc.Login(context.Background(), "a@example.com", testPassword, "ip")
	require.NoError(t, err)
	assert.Empty(t, log.Entries(), "Login writes nothing on success; StartSession writes LOGIN")
}

// The response must not change when the audit write fails: the client gets the
// usual refusal, and the failure goes to the server log.
func TestLogin_AuditFailureKeepsTheRefusal(t *testing.T) {
	repo := userimpl.NewMockStore()
	log := auditimpl.NewMockStore()
	log.InsertFunc = func(context.Context, audit.Log) error { return errors.New("db down") }
	svc := newService(repo, nil, nil, log, nil, discardLog)
	seedUser(t, repo, "a@example.com", "Ana")

	_, err := svc.Login(context.Background(), "a@example.com", "wrong-password", "ip")
	assert.ErrorIs(t, err, auth.ErrInvalidCredentials)
	_, err = svc.Login(context.Background(), "nobody@example.com", testPassword, "ip")
	assert.ErrorIs(t, err, auth.ErrInvalidCredentials)
}

func TestStartSession_AuditsLogin(t *testing.T) {
	f := newTokenFixture(t, defaultSet)

	_, err := f.svc.StartSession(context.Background(), f.userID, "10.0.0.9", "ua")
	require.NoError(t, err)

	rows := f.audit.Entries()
	require.Len(t, rows, 1)
	e := rows[0]
	assert.Equal(t, audit.ActionLogin, e.Action)
	assert.Equal(t, f.userID, *e.PerformedBy)
	assert.Equal(t, "Ana", *e.PerformedByName)
	assert.Equal(t, strconv.FormatInt(f.userID, 10), *e.TargetID)
	assert.Equal(t, "ana@example.com", *e.TargetName)
	assert.Empty(t, e.Details)
}

func TestStartSession_FailsWhenTheAuditFails(t *testing.T) {
	f := newTokenFixture(t, defaultSet)
	f.audit.InsertFunc = func(context.Context, audit.Log) error { return errors.New("check violation") }

	_, err := f.svc.StartSession(context.Background(), f.userID, "ip", "ua")
	require.Error(t, err, "a login without its audit row must not start a session")
}

func TestEndSession_AuditsOnlyALiveSession(t *testing.T) {
	f := newTokenFixture(t, defaultSet)
	ctx := context.Background()
	p, err := f.svc.StartSession(ctx, f.userID, "ip", "ua")
	require.NoError(t, err)

	require.NoError(t, f.svc.EndSession(ctx, p.Refresh, "10.0.0.9"))
	require.NoError(t, f.svc.EndSession(ctx, p.Refresh, "10.0.0.9"), "a second logout is fine")
	require.NoError(t, f.svc.EndSession(ctx, "made-up", "10.0.0.9"), "an unknown token is fine")

	var logouts []audit.Log
	for _, e := range f.audit.Entries() {
		if e.Action == audit.ActionLogout {
			logouts = append(logouts, e)
		}
	}
	require.Len(t, logouts, 1, "only the logout that ended a live session is audited")
	assert.Equal(t, f.userID, *logouts[0].PerformedBy)
	assert.Equal(t, strconv.FormatInt(f.userID, 10), *logouts[0].TargetID)
}
