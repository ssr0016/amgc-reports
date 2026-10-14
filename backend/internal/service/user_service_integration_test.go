//go:build integration

package service

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ssr0016/pastoral-reports/backend/internal/database"
	"github.com/ssr0016/pastoral-reports/backend/internal/model"
	"github.com/ssr0016/pastoral-reports/backend/internal/repository"
	"github.com/ssr0016/pastoral-reports/backend/internal/testutil"
)

// failingAudit makes every audit insert fail, as a database error would.
type failingAudit struct{}

func (failingAudit) Insert(context.Context, repository.Querier, model.AuditLog) error {
	return errors.New("audit insert failed")
}

type userSvcEnv struct {
	tdb   *testutil.TestDB
	users *repository.UserRepo
	roles repository.RoleRepository
	admin *model.User
}

func newUserSvcEnv(t *testing.T) userSvcEnv {
	t.Helper()
	tdb := testutil.SetupPostgres(t)
	tdb.MigrateWithGoose(t) // the real migrations: audit_logs and its CHECK come from them
	db := &database.DB{Pool: tdb.Pool, Builder: database.NewStatementBuilder()}
	users := repository.NewUserRepo(db)
	admin, err := users.CreateWithPassword(context.Background(), "admin@example.com", "Admin", "hash")
	require.NoError(t, err)
	return userSvcEnv{tdb: tdb, users: users, roles: repository.NewRoleRepo(db), admin: admin}
}

func (e userSvcEnv) service(audit repository.AuditRepository) *UserService {
	return NewUserService(e.users, e.roles, audit, repository.NewTxManager(e.tdb.Pool))
}

func (e userSvcEnv) auditActions(t *testing.T, action string) int {
	t.Helper()
	var n int
	require.NoError(t, e.tdb.Pool.QueryRow(context.Background(),
		`SELECT count(*) FROM audit_logs WHERE action = $1`, action).Scan(&n))
	return n
}

func TestUserService_Create_WritesAccountAndAuditInOneTransaction(t *testing.T) {
	env := newUserSvcEnv(t)
	ctx := context.Background()
	svc := env.service(repository.NewAuditRepo(&database.DB{Pool: env.tdb.Pool, Builder: database.NewStatementBuilder()}))

	got, err := svc.Create(ctx, env.admin.ID, "10.0.0.9", createReq())
	require.NoError(t, err)

	stored, err := env.users.GetByID(ctx, got.ID)
	require.NoError(t, err)
	require.NotNil(t, stored)
	assert.Equal(t, "Pastor Pia", *stored.Worker)
	assert.Equal(t, "AMGC 1", *stored.ChurchName)

	var performedBy int64
	var targetType, targetName, details string
	require.NoError(t, env.tdb.Pool.QueryRow(ctx,
		`SELECT performed_by, target_type, target_name, details::text FROM audit_logs WHERE action = 'CREATE_USER'`).
		Scan(&performedBy, &targetType, &targetName, &details), "the CHECK must accept CREATE_USER")
	assert.Equal(t, env.admin.ID, performedBy)
	assert.Equal(t, "User", targetType)
	assert.Equal(t, "pastor@example.com", targetName)
	assert.NotContains(t, details, "hash")
}

func TestUserService_Create_AuditFailureRollsBackTheAccount(t *testing.T) {
	env := newUserSvcEnv(t)
	ctx := context.Background()

	_, err := env.service(failingAudit{}).Create(ctx, env.admin.ID, "", createReq())
	require.ErrorContains(t, err, "audit insert failed")

	gone, err := env.users.GetByEmail(ctx, "pastor@example.com")
	require.NoError(t, err)
	assert.Nil(t, gone, "the account must not exist when its audit row could not be written")
}

func TestUserService_Create_DuplicateEmailAndUnknownRole(t *testing.T) {
	env := newUserSvcEnv(t)
	ctx := context.Background()
	svc := env.service(repository.NewAuditRepo(&database.DB{Pool: env.tdb.Pool, Builder: database.NewStatementBuilder()}))

	_, err := svc.Create(ctx, env.admin.ID, "", createReq())
	require.NoError(t, err)
	_, err = svc.Create(ctx, env.admin.ID, "", createReq())
	assert.ErrorIs(t, err, ErrEmailTaken)

	req := createReq()
	req.Email = "other@example.com"
	missing := int64(999999)
	req.RoleID = &missing
	_, err = svc.Create(ctx, env.admin.ID, "", req)
	assert.ErrorIs(t, err, ErrRoleNotFound)
	assert.Equal(t, 1, env.auditActions(t, "CREATE_USER"), "only the successful create is audited")
}

func TestUserService_Update_WritesProfileAndAudit(t *testing.T) {
	env := newUserSvcEnv(t)
	ctx := context.Background()
	svc := env.service(repository.NewAuditRepo(&database.DB{Pool: env.tdb.Pool, Builder: database.NewStatementBuilder()}))
	created, err := svc.Create(ctx, env.admin.ID, "", createReq())
	require.NoError(t, err)

	got, err := svc.Update(ctx, env.admin.ID, "", created.ID, model.UpdateUserRequest{Name: "Pia Renamed", Worker: "Pastor Pia 2", AreaAssignment: "", ChurchName: "AMGC 2"})
	require.NoError(t, err)
	assert.Equal(t, "Pia Renamed", got.Name)
	assert.Nil(t, got.AreaAssignment)
	assert.Equal(t, 1, env.auditActions(t, "UPDATE_USER"))

	_, err = svc.Update(ctx, env.admin.ID, "", 999999, model.UpdateUserRequest{Name: "Nobody"})
	assert.ErrorIs(t, err, ErrUserNotFound)
	assert.Equal(t, 1, env.auditActions(t, "UPDATE_USER"), "a failed update writes no audit row")
}
