//go:build integration

package userimpl

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ssr0016/pastoral-reports/backend/internal/audit"
	"github.com/ssr0016/pastoral-reports/backend/internal/audit/auditimpl"
	"github.com/ssr0016/pastoral-reports/backend/internal/database"
	"github.com/ssr0016/pastoral-reports/backend/internal/user"
)

func (e userSvcEnv) realAudit() audit.Store {
	return auditimpl.NewStore(&database.DB{Pool: e.tdb.Pool, Builder: database.NewStatementBuilder()})
}

func TestUserService_Delete_WritesMarkSessionsAndAuditInOneTransaction(t *testing.T) {
	env := newUserSvcEnv(t)
	ctx := context.Background()
	actor := env.newAdmin(t, "actor@example.com", env.adminRole(t))
	target, err := env.users.CreateWithPassword(ctx, "target@example.com", "Target", "h")
	require.NoError(t, err)
	env.addSessions(t, target.ID, "t1", "t2")

	got, err := env.service(env.realAudit()).Delete(ctx, actor.ID, "10.0.0.9", target.ID, " left ")
	require.NoError(t, err)
	require.NotNil(t, got.DeletedAt)
	assert.Equal(t, user.UserStatusActive, got.Status, "delete does not touch the status")
	assert.Zero(t, env.liveSessions(t, target.ID), "the deleted user's refresh tokens must be revoked")

	var deletedBy int64
	var reason string
	require.NoError(t, env.tdb.Pool.QueryRow(ctx,
		`SELECT deleted_by, deleted_reason FROM users WHERE id = $1`, target.ID).Scan(&deletedBy, &reason))
	assert.Equal(t, actor.ID, deletedBy)
	assert.Equal(t, "left", reason)

	var details string
	require.NoError(t, env.tdb.Pool.QueryRow(ctx,
		`SELECT details::text FROM audit_logs WHERE action = 'DELETE_USER'`).Scan(&details),
		"the CHECK must accept DELETE_USER")
	assert.JSONEq(t, `{"reason":"left","sessions_revoked":2}`, details)
}

func TestUserService_Delete_AuditFailureRollsBackMarkAndSessions(t *testing.T) {
	env := newUserSvcEnv(t)
	ctx := context.Background()
	actor := env.newAdmin(t, "actor@example.com", env.adminRole(t))
	target, err := env.users.CreateWithPassword(ctx, "target@example.com", "Target", "h")
	require.NoError(t, err)
	env.addSessions(t, target.ID, "t1")

	_, err = env.service(failingAudit{}).Delete(ctx, actor.ID, "", target.ID, "")
	require.Error(t, err)

	stored, err := env.users.GetByID(ctx, target.ID)
	require.NoError(t, err)
	assert.Nil(t, stored.DeletedAt, "the delete must roll back without its audit row")
	assert.Equal(t, 1, env.liveSessions(t, target.ID), "the sessions must survive a rolled-back delete")
}

func TestUserService_Restore_WritesAndAuditsInOneTransaction(t *testing.T) {
	env := newUserSvcEnv(t)
	ctx := context.Background()
	actor := env.newAdmin(t, "actor@example.com", env.adminRole(t))
	target, err := env.users.CreateWithPassword(ctx, "target@example.com", "Target", "h")
	require.NoError(t, err)
	svc := env.service(env.realAudit())
	_, err = svc.Delete(ctx, actor.ID, "", target.ID, "mistake")
	require.NoError(t, err)

	// A failing audit rolls the restore back.
	_, err = env.service(failingAudit{}).Restore(ctx, actor.ID, "", target.ID)
	require.Error(t, err)
	stored, err := env.users.GetByID(ctx, target.ID)
	require.NoError(t, err)
	assert.NotNil(t, stored.DeletedAt, "the restore must roll back without its audit row")

	got, err := svc.Restore(ctx, actor.ID, "", target.ID)
	require.NoError(t, err)
	assert.Nil(t, got.DeletedAt)
	assert.Nil(t, got.DeletedBy)
	assert.Nil(t, got.DeletedReason)
	assert.True(t, got.CanSignIn(), "a restored active user can sign in again")

	var performedBy, deletedBy int64
	var reason string
	require.NoError(t, env.tdb.Pool.QueryRow(ctx,
		`SELECT performed_by, (details->>'deleted_by')::bigint, details->>'deleted_reason'
		 FROM audit_logs WHERE action = 'RESTORE_USER'`).Scan(&performedBy, &deletedBy, &reason),
		"the CHECK must accept RESTORE_USER")
	assert.Equal(t, actor.ID, performedBy)
	assert.Equal(t, actor.ID, deletedBy)
	assert.Equal(t, "mistake", reason)
}

func TestUserService_Create_DeletedUsersEmailIsReserved(t *testing.T) {
	env := newUserSvcEnv(t)
	ctx := context.Background()
	actor := env.newAdmin(t, "actor@example.com", env.adminRole(t))
	target, err := env.users.CreateWithPassword(ctx, "pastor@example.com", "Old", "h")
	require.NoError(t, err)
	svc := env.service(env.realAudit())
	_, err = svc.Delete(ctx, actor.ID, "", target.ID, "")
	require.NoError(t, err)

	_, err = svc.Create(ctx, actor.ID, "", createReq())
	assert.ErrorIs(t, err, user.ErrEmailBelongsToDeletedUser)
}

func TestUserRepo_List_DeletedFilter(t *testing.T) {
	env := newUserSvcEnv(t)
	ctx := context.Background()
	gone, err := env.users.CreateWithPassword(ctx, "gone@example.com", "Gone", "h")
	require.NoError(t, err)
	_, err = env.users.SoftDelete(ctx, gone.ID, env.admin.ID, "")
	require.NoError(t, err)

	emails := func(status string) []string {
		list, err := env.users.List(ctx, "", status, 100)
		require.NoError(t, err)
		var out []string
		for _, u := range list {
			out = append(out, u.Email)
		}
		return out
	}
	assert.Equal(t, []string{"admin@example.com"}, emails(""), `"" leaves deleted users out`)
	assert.Equal(t, []string{"gone@example.com"}, emails("deleted"), "deleted returns only deleted users")
	assert.Equal(t, []string{"admin@example.com"}, emails(user.UserStatusActive), "a deleted active user is not active in the list")
}

func TestUserRepo_LockActiveAdmins_SkipsADeletedAdmin(t *testing.T) {
	env := newUserSvcEnv(t)
	ctx := context.Background()
	role := env.adminRole(t)
	a := env.newAdmin(t, "a@example.com", role)
	b := env.newAdmin(t, "b@example.com", role)
	_, err := env.users.SoftDelete(ctx, b.ID, a.ID, "")
	require.NoError(t, err)

	ids, err := env.users.LockActiveAdmins(ctx)
	require.NoError(t, err)
	assert.Equal(t, []int64{a.ID}, ids)
}

func TestUserRepo_SoftDeleteAndRestore_MissingUser(t *testing.T) {
	env := newUserSvcEnv(t)
	ctx := context.Background()
	_, err := env.users.SoftDelete(ctx, 999999, env.admin.ID, "")
	assert.ErrorIs(t, err, database.ErrNotFound)
	_, err = env.users.Restore(ctx, 999999)
	assert.ErrorIs(t, err, database.ErrNotFound)
}
