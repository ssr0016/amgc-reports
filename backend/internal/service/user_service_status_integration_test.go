//go:build integration

package service

import (
	"context"
	"errors"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ssr0016/pastoral-reports/backend/internal/database"
	"github.com/ssr0016/pastoral-reports/backend/internal/model"
	"github.com/ssr0016/pastoral-reports/backend/internal/repository"
)

func (e userSvcEnv) adminRole(t *testing.T) int64 {
	t.Helper()
	var id int64
	require.NoError(t, e.tdb.Pool.QueryRow(context.Background(),
		`INSERT INTO roles (name, description) VALUES ('admin', 'test')
		 ON CONFLICT (name) DO UPDATE SET description = roles.description RETURNING id`).Scan(&id))
	return id
}

func (e userSvcEnv) newAdmin(t *testing.T, email string, roleID int64) *model.User {
	t.Helper()
	u, err := e.users.Create(context.Background(), e.tdb.Pool, model.NewUser{Email: email, Name: "Admin", PasswordHash: "h", RoleID: roleID})
	require.NoError(t, err)
	return u
}

func (e userSvcEnv) addSessions(t *testing.T, userID int64, tokens ...string) {
	t.Helper()
	for _, tok := range tokens {
		_, err := e.tdb.Pool.Exec(context.Background(),
			`INSERT INTO sessions (token, data, expiry, user_id) VALUES ($1, '\x00', NOW() + interval '1 hour', $2)`, tok, userID)
		require.NoError(t, err)
	}
}

func (e userSvcEnv) liveSessions(t *testing.T, userID int64) int {
	t.Helper()
	var n int
	require.NoError(t, e.tdb.Pool.QueryRow(context.Background(),
		`SELECT COUNT(*) FROM sessions WHERE user_id = $1 AND revoked_at IS NULL`, userID).Scan(&n))
	return n
}

func TestUserService_ChangeStatus_WritesStatusSessionsAndAuditInOneTransaction(t *testing.T) {
	env := newUserSvcEnv(t)
	ctx := context.Background()
	actor := env.newAdmin(t, "actor@example.com", env.adminRole(t))
	target, err := env.users.CreateWithPassword(ctx, "target@example.com", "Target", "h")
	require.NoError(t, err)
	env.addSessions(t, target.ID, "t1", "t2")
	svc := env.service(repository.NewAuditRepo(&database.DB{Pool: env.tdb.Pool, Builder: database.NewStatementBuilder()}))

	got, err := svc.ChangeStatus(ctx, actor.ID, "10.0.0.9", target.ID, model.UserStatusDisciplinary)
	require.NoError(t, err)

	assert.Equal(t, model.UserStatusDisciplinary, got.Status)
	assert.Zero(t, env.liveSessions(t, target.ID), "the user's sessions survived")
	var performedBy int64
	var targetName, details string
	require.NoError(t, env.tdb.Pool.QueryRow(ctx,
		`SELECT performed_by, target_name, details::text FROM audit_logs WHERE action = 'CHANGE_USER_STATUS'`).
		Scan(&performedBy, &targetName, &details), "the CHECK must accept CHANGE_USER_STATUS")
	assert.Equal(t, actor.ID, performedBy)
	assert.Equal(t, "target@example.com", targetName)
	assert.JSONEq(t, `{"from":"active","to":"disciplinary","sessions_revoked":2}`, details)
}

func TestUserService_ChangeStatus_AuditFailureRollsBackStatusAndSessions(t *testing.T) {
	env := newUserSvcEnv(t)
	ctx := context.Background()
	actor := env.newAdmin(t, "actor@example.com", env.adminRole(t))
	target, err := env.users.CreateWithPassword(ctx, "target@example.com", "Target", "h")
	require.NoError(t, err)
	env.addSessions(t, target.ID, "t1")

	_, err = env.service(failingAudit{}).ChangeStatus(ctx, actor.ID, "", target.ID, model.UserStatusInactive)

	require.Error(t, err)
	stored, err := env.users.GetByID(ctx, target.ID)
	require.NoError(t, err)
	assert.Equal(t, model.UserStatusActive, stored.Status, "the status change survived a failed audit")
	assert.Equal(t, 1, env.liveSessions(t, target.ID), "the sessions were ended without the change")
}

// Two admins deactivating each other at the same moment must not leave the app
// without an active admin: the row locks make the second check see the first change.
func TestUserService_ChangeStatus_TwoAdminsCannotLockEachOtherOut(t *testing.T) {
	env := newUserSvcEnv(t)
	ctx := context.Background()
	role := env.adminRole(t)
	a := env.newAdmin(t, "a@example.com", role)
	b := env.newAdmin(t, "b@example.com", role)
	svc := env.service(repository.NewAuditRepo(&database.DB{Pool: env.tdb.Pool, Builder: database.NewStatementBuilder()}))

	for round := 0; round < 10; round++ {
		_, err := env.tdb.Pool.Exec(ctx, `UPDATE users SET status = 'active' WHERE id IN ($1, $2)`, a.ID, b.ID)
		require.NoError(t, err)

		var wg sync.WaitGroup
		errs := make([]error, 2)
		start := make(chan struct{})
		for i, pair := range [][2]int64{{a.ID, b.ID}, {b.ID, a.ID}} {
			wg.Add(1)
			go func() {
				defer wg.Done()
				<-start
				_, errs[i] = svc.ChangeStatus(ctx, pair[0], "", pair[1], model.UserStatusInactive)
			}()
		}
		close(start)
		wg.Wait()

		var active int
		require.NoError(t, env.tdb.Pool.QueryRow(ctx,
			`SELECT COUNT(*) FROM users WHERE id IN ($1, $2) AND status = 'active'`, a.ID, b.ID).Scan(&active))
		assert.Equal(t, 1, active, "round %d: exactly one admin must stay active (errors: %v)", round, errs)
		ok, refused := 0, 0
		for _, err := range errs {
			switch {
			case err == nil:
				ok++
			case errors.Is(err, ErrLastActiveAdmin):
				refused++
			}
		}
		assert.Equal(t, 1, ok, "round %d: one change goes through", round)
		assert.Equal(t, 1, refused, "round %d: the other is refused as the last admin", round)
	}
}
