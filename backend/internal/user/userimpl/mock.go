package userimpl

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/ssr0016/pastoral-reports/backend/internal/accesscontrol"
	"github.com/ssr0016/pastoral-reports/backend/internal/database"
	"github.com/ssr0016/pastoral-reports/backend/internal/user"
)

// MockStore is an in-memory user.Store for testing.
type MockStore struct {
	mu     sync.Mutex
	users  map[int64]*user.User
	nextID int64

	// Override functions for testing specific behaviors
	GetByIDFunc                   func(ctx context.Context, id int64) (*user.User, error)
	GetByEmailFunc                func(ctx context.Context, email string) (*user.User, error)
	CreateWithPasswordFunc        func(ctx context.Context, email, name, hash string) (*user.User, error)
	CreateFunc                    func(ctx context.Context, n user.NewUser) (*user.User, error)
	UpdateProfileFunc             func(ctx context.Context, id int64, p user.UserProfile) (*user.User, error)
	ListFunc                      func(ctx context.Context, emailFilter, status string, roleID int64, limit int) ([]user.User, error)
	UpdateRoleFunc                func(ctx context.Context, userID, roleID int64) error
	GetWithRoleFunc               func(ctx context.Context, id int64) (*user.User, error)
	ListWithPaginationFunc        func(ctx context.Context, emailFilter string, page, limit int) ([]user.User, int64, error)
	MarkEmailVerifiedFunc         func(ctx context.Context, userID int64) error
	UpdatePasswordFunc            func(ctx context.Context, userID int64, hash string) error
	UpdatePasswordKeepSessionFunc func(ctx context.Context, userID int64, hash, keepToken string) (int64, error)
	RecordFailedLoginFunc         func(ctx context.Context, userID int64, maxAttempts int, lockDuration time.Duration) error
	ResetLoginAttemptsFunc        func(ctx context.Context, userID int64) error
	ClearLockoutFunc              func(ctx context.Context, userID int64) error
	UpdateStatusFunc              func(ctx context.Context, id int64, status string) (*user.User, error)
	RevokeSessionsFunc            func(ctx context.Context, userID int64) (int64, error)
	LockActiveAdminsFunc          func(ctx context.Context) ([]int64, error)
	LockUserFunc                  func(ctx context.Context, userID int64) error
	SoftDeleteFunc                func(ctx context.Context, id, deletedBy int64, reason string) (*user.User, error)
	RestoreFunc                   func(ctx context.Context, id int64) (*user.User, error)
	GetCallerFunc                 func(ctx context.Context, id int64) (*user.Caller, error)

	// Roles, when set, gives GetCaller the role's permissions; nil means none.
	Roles accesscontrol.RoleStore
}

// NewMockStore creates a new mock repository.
func NewMockStore() *MockStore {
	return &MockStore{
		users:  make(map[int64]*user.User),
		nextID: 1,
	}
}

func (m *MockStore) GetByID(ctx context.Context, id int64) (*user.User, error) {
	if m.GetByIDFunc != nil {
		return m.GetByIDFunc(ctx, id)
	}
	m.mu.Lock()
	defer m.mu.Unlock()

	usr, ok := m.users[id]
	if !ok {
		return nil, nil
	}
	return usr, nil
}

func (m *MockStore) GetByEmail(ctx context.Context, email string) (*user.User, error) {
	if m.GetByEmailFunc != nil {
		return m.GetByEmailFunc(ctx, email)
	}
	m.mu.Lock()
	defer m.mu.Unlock()

	for _, u := range m.users {
		if u.Email == email {
			return u, nil
		}
	}
	return nil, nil
}

func (m *MockStore) CreateWithPassword(ctx context.Context, email, name, hash string) (*user.User, error) {
	if m.CreateWithPasswordFunc != nil {
		return m.CreateWithPasswordFunc(ctx, email, name, hash)
	}
	m.mu.Lock()
	defer m.mu.Unlock()

	usr := &user.User{
		ID:           m.nextID,
		Email:        email,
		Name:         name,
		PasswordHash: hash,
		Status:       user.UserStatusActive,
	}
	m.users[m.nextID] = usr
	m.nextID++
	return usr, nil
}

// Create inserts an account (mock).
func (m *MockStore) Create(ctx context.Context, n user.NewUser) (*user.User, error) {
	if m.CreateFunc != nil {
		return m.CreateFunc(ctx, n)
	}
	m.mu.Lock()
	defer m.mu.Unlock()

	for _, u := range m.users {
		if u.Email == n.Email {
			return nil, fmt.Errorf("users: %w", database.ErrUniqueViolation)
		}
	}
	usr := &user.User{
		ID: m.nextID, Email: n.Email, Name: n.Name, PasswordHash: n.PasswordHash, RoleID: n.RoleID,
		AreaAssignment: nullable(n.AreaAssignment), ChurchName: nullable(n.ChurchName),
		Status: user.UserStatusActive,
	}
	m.users[m.nextID] = usr
	m.nextID++
	return usr, nil
}

// UpdateProfile replaces a user's name and profile text (mock).
func (m *MockStore) UpdateProfile(ctx context.Context, id int64, p user.UserProfile) (*user.User, error) {
	if m.UpdateProfileFunc != nil {
		return m.UpdateProfileFunc(ctx, id, p)
	}
	m.mu.Lock()
	defer m.mu.Unlock()

	usr, ok := m.users[id]
	if !ok {
		return nil, fmt.Errorf("users: %w", database.ErrNotFound)
	}
	usr.Name = p.Name
	usr.AreaAssignment, usr.ChurchName = nullable(p.AreaAssignment), nullable(p.ChurchName)
	return usr, nil
}

func (m *MockStore) List(ctx context.Context, emailFilter, status string, roleID int64, limit int) ([]user.User, error) {
	if m.ListFunc != nil {
		return m.ListFunc(ctx, emailFilter, status, roleID, limit)
	}
	m.mu.Lock()
	defer m.mu.Unlock()

	result := make([]user.User, 0, len(m.users))
	for _, u := range m.users {
		result = append(result, *u)
	}
	return result, nil
}

// ReassignRole moves every user holding from to to (mock).
func (m *MockStore) ReassignRole(_ context.Context, from, to int64) ([]user.User, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	var moved []user.User
	for _, u := range m.users {
		if u.RoleID == from {
			u.RoleID = to
			moved = append(moved, *u)
		}
	}
	return moved, nil
}

// UpdateRole changes a user's role (mock).
func (m *MockStore) UpdateRole(ctx context.Context, userID, roleID int64) error {
	if m.UpdateRoleFunc != nil {
		return m.UpdateRoleFunc(ctx, userID, roleID)
	}
	m.mu.Lock()
	defer m.mu.Unlock()

	user, ok := m.users[userID]
	if !ok {
		return fmt.Errorf("users: %w", database.ErrNotFound)
	}
	user.RoleID = roleID
	return nil
}

// GetWithRole returns a user with role (mock).
func (m *MockStore) GetWithRole(ctx context.Context, id int64) (*user.User, error) {
	if m.GetWithRoleFunc != nil {
		return m.GetWithRoleFunc(ctx, id)
	}
	m.mu.Lock()
	defer m.mu.Unlock()

	usr, ok := m.users[id]
	if !ok {
		return nil, nil
	}
	return usr, nil
}

// GetCaller is GetWithRole plus Roles.GetPermissions for the user's role (mock).
func (m *MockStore) GetCaller(ctx context.Context, id int64) (*user.Caller, error) {
	if m.GetCallerFunc != nil {
		return m.GetCallerFunc(ctx, id)
	}
	u, err := m.GetWithRole(ctx, id)
	if err != nil || u == nil {
		return nil, err
	}
	c := &user.Caller{User: u, Permissions: map[string]bool{}}
	if m.Roles != nil && u.RoleID != 0 {
		perms, err := m.Roles.GetPermissions(ctx, u.RoleID)
		if err != nil {
			return nil, err
		}
		for _, p := range perms {
			c.Permissions[p.Name] = true
		}
	}
	return c, nil
}

// ListWithPagination returns users with pagination (mock).
func (m *MockStore) ListWithPagination(ctx context.Context, emailFilter string, page, limit int) ([]user.User, int64, error) {
	if m.ListWithPaginationFunc != nil {
		return m.ListWithPaginationFunc(ctx, emailFilter, page, limit)
	}
	m.mu.Lock()
	defer m.mu.Unlock()

	result := make([]user.User, 0, len(m.users))
	for _, u := range m.users {
		result = append(result, *u)
	}
	return result, int64(len(result)), nil
}

// MarkEmailVerified marks email as verified (mock).
func (m *MockStore) MarkEmailVerified(ctx context.Context, userID int64) error {
	if m.MarkEmailVerifiedFunc != nil {
		return m.MarkEmailVerifiedFunc(ctx, userID)
	}
	m.mu.Lock()
	defer m.mu.Unlock()

	user, ok := m.users[userID]
	if !ok {
		return fmt.Errorf("users: %w", database.ErrNotFound)
	}
	user.EmailVerified = true
	now := time.Now()
	user.EmailVerifiedAt = &now
	return nil
}

// UpdatePassword updates a user's password (mock).
func (m *MockStore) UpdatePassword(ctx context.Context, userID int64, hash string) error {
	if m.UpdatePasswordFunc != nil {
		return m.UpdatePasswordFunc(ctx, userID, hash)
	}
	m.mu.Lock()
	defer m.mu.Unlock()

	user, ok := m.users[userID]
	if !ok {
		return fmt.Errorf("users: %w", database.ErrNotFound)
	}
	user.PasswordHash = hash
	return nil
}

// UpdatePasswordKeepSession sets the password (mock); the mock has no sessions, so it ends none.
func (m *MockStore) UpdatePasswordKeepSession(ctx context.Context, userID int64, hash, keepToken string) (int64, error) {
	if m.UpdatePasswordKeepSessionFunc != nil {
		return m.UpdatePasswordKeepSessionFunc(ctx, userID, hash, keepToken)
	}
	return 0, m.UpdatePassword(ctx, userID, hash)
}

// RecordFailedLogin increments failed attempts (mock).
func (m *MockStore) RecordFailedLogin(ctx context.Context, userID int64, maxAttempts int, lockDuration time.Duration) error {
	if m.RecordFailedLoginFunc != nil {
		return m.RecordFailedLoginFunc(ctx, userID, maxAttempts, lockDuration)
	}
	m.mu.Lock()
	defer m.mu.Unlock()

	user, ok := m.users[userID]
	if !ok {
		return fmt.Errorf("users: %w", database.ErrNotFound)
	}
	user.FailedLoginAttempts++
	if user.FailedLoginAttempts >= maxAttempts {
		lockedUntil := time.Now().Add(lockDuration)
		user.LockedUntil = &lockedUntil
	}
	return nil
}

// ResetLoginAttempts clears failed attempts (mock).
func (m *MockStore) ResetLoginAttempts(ctx context.Context, userID int64) error {
	if m.ResetLoginAttemptsFunc != nil {
		return m.ResetLoginAttemptsFunc(ctx, userID)
	}
	m.mu.Lock()
	defer m.mu.Unlock()

	user, ok := m.users[userID]
	if !ok {
		return fmt.Errorf("users: %w", database.ErrNotFound)
	}
	user.FailedLoginAttempts = 0
	user.LockedUntil = nil
	return nil
}

// ClearLockout clears failed attempts and the lock (mock).
func (m *MockStore) ClearLockout(ctx context.Context, userID int64) error {
	if m.ClearLockoutFunc != nil {
		return m.ClearLockoutFunc(ctx, userID)
	}
	return m.ResetLoginAttempts(ctx, userID)
}

// UpdateStatus sets a user's status (mock).
func (m *MockStore) UpdateStatus(ctx context.Context, id int64, status string) (*user.User, error) {
	if m.UpdateStatusFunc != nil {
		return m.UpdateStatusFunc(ctx, id, status)
	}
	m.mu.Lock()
	defer m.mu.Unlock()

	usr, ok := m.users[id]
	if !ok {
		return nil, fmt.Errorf("users: %w", database.ErrNotFound)
	}
	usr.Status = status
	return usr, nil
}

// RevokeSessions ends a user's sessions (mock): there are none, so it ends none.
func (m *MockStore) RevokeSessions(ctx context.Context, userID int64) (int64, error) {
	if m.RevokeSessionsFunc != nil {
		return m.RevokeSessionsFunc(ctx, userID)
	}
	return 0, nil
}

// LockUser takes no lock (mock) unless LockUserFunc is set.
func (m *MockStore) LockUser(ctx context.Context, userID int64) error {
	if m.LockUserFunc != nil {
		return m.LockUserFunc(ctx, userID)
	}
	return nil
}

// LockActiveAdmins returns the ids of the active users whose role id is the mock
// admin role, in id order (mock). The mock has no role names, so it returns none
// unless LockActiveAdminsFunc is set.
func (m *MockStore) LockActiveAdmins(ctx context.Context) ([]int64, error) {
	if m.LockActiveAdminsFunc != nil {
		return m.LockActiveAdminsFunc(ctx)
	}
	return nil, nil
}

// SoftDelete marks a user deleted (mock).
func (m *MockStore) SoftDelete(ctx context.Context, id, deletedBy int64, reason string) (*user.User, error) {
	if m.SoftDeleteFunc != nil {
		return m.SoftDeleteFunc(ctx, id, deletedBy, reason)
	}
	m.mu.Lock()
	defer m.mu.Unlock()

	usr, ok := m.users[id]
	if !ok {
		return nil, fmt.Errorf("users: %w", database.ErrNotFound)
	}
	now := time.Now()
	usr.DeletedAt, usr.DeletedBy, usr.DeletedReason = &now, &deletedBy, nullable(reason)
	return usr, nil
}

// Restore clears a user's deleted mark (mock).
func (m *MockStore) Restore(ctx context.Context, id int64) (*user.User, error) {
	if m.RestoreFunc != nil {
		return m.RestoreFunc(ctx, id)
	}
	m.mu.Lock()
	defer m.mu.Unlock()

	usr, ok := m.users[id]
	if !ok {
		return nil, fmt.Errorf("users: %w", database.ErrNotFound)
	}
	usr.DeletedAt, usr.DeletedBy, usr.DeletedReason = nil, nil, nil
	return usr, nil
}
