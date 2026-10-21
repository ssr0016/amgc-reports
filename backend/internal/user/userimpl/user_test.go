package userimpl

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"golang.org/x/crypto/bcrypt"

	"github.com/ssr0016/pastoral-reports/backend/internal/accesscontrol"
	"github.com/ssr0016/pastoral-reports/backend/internal/audit"
	"github.com/ssr0016/pastoral-reports/backend/internal/audit/auditimpl"
	"github.com/ssr0016/pastoral-reports/backend/internal/auth"
	"github.com/ssr0016/pastoral-reports/backend/internal/database"
	"github.com/ssr0016/pastoral-reports/backend/internal/user"
)

const adminID = int64(99)

// roleSet answers GetByID for the listed role IDs; anything else is "not found" (nil).
type roleSet map[int64]bool

type stubRoleLookup struct {
	accesscontrol.RoleStore
	known roleSet
}

func (s stubRoleLookup) GetByID(_ context.Context, id int64) (*accesscontrol.Role, error) {
	if s.known[id] {
		return &accesscontrol.Role{ID: id, Name: fmt.Sprintf("role-%d", id)}, nil
	}
	return nil, nil
}

type userFixture struct {
	svc   user.Service
	users *MockStore
	audit *auditimpl.MockStore
}

func newUserFixture() userFixture {
	users := NewMockStore()
	users.GetWithRoleFunc = func(_ context.Context, id int64) (*user.User, error) {
		if id == adminID {
			return &user.User{ID: adminID, Name: "Admin", Role: &accesscontrol.Role{Name: "admin"}}, nil
		}
		return nil, nil
	}
	f := userFixture{users: users, audit: auditimpl.NewMockStore()}
	f.svc = NewService(users, stubRoleLookup{known: roleSet{7: true}}, f.audit, fakeTx{})
	return f
}

func createReq() user.CreateUserRequest {
	return user.CreateUserRequest{
		Email: "pastor@example.com", Name: "Pia", Password: testPassword,
		AreaAssignment: "Metro", ChurchName: "AMGC 1",
	}
}

func TestUserService_Create(t *testing.T) {
	ctx := context.Background()

	t.Run("creates the account, hashes the password and writes one audit row", func(t *testing.T) {
		f := newUserFixture()
		got, err := f.svc.Create(ctx, adminID, "10.0.0.1", createReq())
		require.NoError(t, err)

		assert.Equal(t, "pastor@example.com", got.Email)
		require.NotNil(t, got.AreaAssignment)
		assert.Equal(t, "Metro", *got.AreaAssignment)
		assert.NoError(t, bcrypt.CompareHashAndPassword([]byte(got.PasswordHash), []byte(testPassword)))
		assert.Zero(t, got.RoleID, "no role_id means the repository picks the default role")

		entries := f.audit.Entries()
		require.Len(t, entries, 1)
		e := entries[0]
		assert.Equal(t, audit.ActionCreateUser, e.Action)
		assert.Equal(t, audit.TargetTypeUser, e.TargetType)
		assert.Equal(t, adminID, *e.PerformedBy)
		assert.Equal(t, "Admin", *e.PerformedByName)
		assert.Equal(t, "admin", *e.PerformedByRole)
		assert.Equal(t, "pastor@example.com", *e.TargetName)
		assert.Equal(t, "10.0.0.1", *e.IPAddress)
		assert.NotContains(t, fmt.Sprint(e), got.PasswordHash, "the audit row must not carry the hash")
		assert.NotContains(t, strings.ToLower(fmt.Sprint(e.Details)), "password")
	})

	t.Run("an existing role_id is used and recorded", func(t *testing.T) {
		f := newUserFixture()
		req := createReq()
		role := int64(7)
		req.RoleID = &role
		got, err := f.svc.Create(ctx, adminID, "", req)
		require.NoError(t, err)
		assert.Equal(t, int64(7), got.RoleID)
		assert.EqualValues(t, 7, f.audit.Entries()[0].Details["role_id"])
	})

	tests := []struct {
		name   string
		setup  func(f userFixture, req *user.CreateUserRequest)
		actor  int64
		want   error
		wantFn func(t *testing.T, err error)
	}{
		{
			name: "unknown role", actor: adminID, want: user.ErrRoleNotFound,
			setup: func(_ userFixture, req *user.CreateUserRequest) { r := int64(404); req.RoleID = &r },
		},
		{
			name: "email already taken", actor: adminID, want: user.ErrEmailTaken,
			setup: func(f userFixture, _ *user.CreateUserRequest) {
				_, _ = f.users.CreateWithPassword(context.Background(), "pastor@example.com", "Old", "h")
			},
		},
		{
			name: "email lookup fails", actor: adminID,
			setup: func(f userFixture, _ *user.CreateUserRequest) {
				f.users.GetByEmailFunc = func(context.Context, string) (*user.User, error) { return nil, errors.New("db error") }
			},
			wantFn: func(t *testing.T, err error) {
				assert.ErrorContains(t, err, "db error")
				assert.NotErrorIs(t, err, user.ErrEmailTaken)
			},
		},
		{
			name: "two creates race: the unique index rejects the second", actor: adminID, want: user.ErrEmailTaken,
			setup: func(f userFixture, _ *user.CreateUserRequest) {
				f.users.CreateFunc = func(context.Context, user.NewUser) (*user.User, error) {
					return nil, fmt.Errorf("users: %w", database.ErrUniqueViolation)
				}
			},
		},
		{
			name: "role deleted between the check and the insert", actor: adminID, want: user.ErrRoleNotFound,
			setup: func(f userFixture, req *user.CreateUserRequest) {
				r := int64(7)
				req.RoleID = &r
				f.users.CreateFunc = func(context.Context, user.NewUser) (*user.User, error) {
					return nil, fmt.Errorf("users: %w", database.ErrForeignKeyViolation)
				}
			},
		},
		{
			name: "the actor no longer exists", actor: 12345, want: auth.ErrUnauthorized,
			setup: func(userFixture, *user.CreateUserRequest) {},
		},
		{
			name: "audit failure fails the whole create", actor: adminID,
			setup: func(f userFixture, _ *user.CreateUserRequest) {
				f.audit.InsertFunc = func(context.Context, audit.Log) error { return errors.New("audit down") }
			},
			wantFn: func(t *testing.T, err error) { assert.ErrorContains(t, err, "audit down") },
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			f := newUserFixture()
			req := createReq()
			tt.setup(f, &req)
			got, err := f.svc.Create(ctx, tt.actor, "", req)
			assert.Nil(t, got)
			if tt.want != nil {
				assert.ErrorIs(t, err, tt.want)
			} else {
				require.Error(t, err)
				tt.wantFn(t, err)
			}
		})
	}
}

func TestUserService_Update(t *testing.T) {
	ctx := context.Background()
	newUser := func(f userFixture) *user.User {
		u, err := f.users.Create(ctx, user.NewUser{Email: "u@example.com", Name: "Old", PasswordHash: "keep", AreaAssignment: "A"})
		require.NoError(t, err)
		return u
	}
	req := user.UpdateUserRequest{Name: "New Name", AreaAssignment: "North", ChurchName: ""}

	t.Run("replaces name and profile and writes one audit row", func(t *testing.T) {
		f := newUserFixture()
		u := newUser(f)
		got, err := f.svc.Update(ctx, adminID, "10.0.0.2", u.ID, req)
		require.NoError(t, err)

		assert.Equal(t, "New Name", got.Name)
		assert.Equal(t, "North", *got.AreaAssignment)
		assert.Nil(t, got.ChurchName, "empty text is cleared")
		assert.Equal(t, "keep", got.PasswordHash)

		entries := f.audit.Entries()
		require.Len(t, entries, 1)
		assert.Equal(t, audit.ActionUpdateUser, entries[0].Action)
		assert.Equal(t, fmt.Sprint(u.ID), *entries[0].TargetID)
		assert.Equal(t, adminID, *entries[0].PerformedBy)
	})

	t.Run("unknown user writes nothing", func(t *testing.T) {
		f := newUserFixture()
		got, err := f.svc.Update(ctx, adminID, "", 404, req)
		assert.Nil(t, got)
		assert.ErrorIs(t, err, user.ErrUserNotFound)
		assert.Empty(t, f.audit.Entries())
	})

	t.Run("audit failure fails the update", func(t *testing.T) {
		f := newUserFixture()
		u := newUser(f)
		f.audit.InsertFunc = func(context.Context, audit.Log) error { return errors.New("audit down") }
		_, err := f.svc.Update(ctx, adminID, "", u.ID, req)
		assert.ErrorContains(t, err, "audit down")
	})

	t.Run("the actor no longer exists", func(t *testing.T) {
		f := newUserFixture()
		u := newUser(f)
		_, err := f.svc.Update(ctx, 12345, "", u.ID, req)
		assert.ErrorIs(t, err, auth.ErrUnauthorized)
	})
}
