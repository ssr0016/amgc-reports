package repository

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/ssr0016/template/internal/model"
)

// MockUserRepo is an in-memory implementation of UserRepository for testing.
type MockUserRepo struct {
	mu     sync.Mutex
	users  map[int64]*model.User
	nextID int64

	// Override functions for testing specific behaviors
	GetByIDFunc            func(ctx context.Context, id int64) (*model.User, error)
	GetByEmailFunc         func(ctx context.Context, email string) (*model.User, error)
	CreateWithPasswordFunc func(ctx context.Context, email, name, hash string) (*model.User, error)
	ListFunc               func(ctx context.Context, emailFilter string, limit int) ([]model.User, error)
	UpdateRoleFunc         func(ctx context.Context, userID, roleID int64) error
	GetWithRoleFunc        func(ctx context.Context, id int64) (*model.User, error)
	ListWithPaginationFunc func(ctx context.Context, emailFilter string, page, limit int) ([]model.User, int64, error)
	MarkEmailVerifiedFunc  func(ctx context.Context, userID int64) error
	UpdatePasswordFunc     func(ctx context.Context, userID int64, hash string) error
	RecordFailedLoginFunc  func(ctx context.Context, userID int64, maxAttempts int, lockDuration time.Duration) error
	ResetLoginAttemptsFunc func(ctx context.Context, userID int64) error
}

// NewMockUserRepo creates a new mock repository.
func NewMockUserRepo() *MockUserRepo {
	return &MockUserRepo{
		users:  make(map[int64]*model.User),
		nextID: 1,
	}
}

func (m *MockUserRepo) GetByID(ctx context.Context, id int64) (*model.User, error) {
	if m.GetByIDFunc != nil {
		return m.GetByIDFunc(ctx, id)
	}
	m.mu.Lock()
	defer m.mu.Unlock()

	user, ok := m.users[id]
	if !ok {
		return nil, nil
	}
	return user, nil
}

func (m *MockUserRepo) GetByEmail(ctx context.Context, email string) (*model.User, error) {
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

func (m *MockUserRepo) CreateWithPassword(ctx context.Context, email, name, hash string) (*model.User, error) {
	if m.CreateWithPasswordFunc != nil {
		return m.CreateWithPasswordFunc(ctx, email, name, hash)
	}
	m.mu.Lock()
	defer m.mu.Unlock()

	user := &model.User{
		ID:           m.nextID,
		Email:        email,
		Name:         name,
		PasswordHash: hash,
	}
	m.users[m.nextID] = user
	m.nextID++
	return user, nil
}

func (m *MockUserRepo) List(ctx context.Context, emailFilter string, limit int) ([]model.User, error) {
	if m.ListFunc != nil {
		return m.ListFunc(ctx, emailFilter, limit)
	}
	m.mu.Lock()
	defer m.mu.Unlock()

	result := make([]model.User, 0, len(m.users))
	for _, u := range m.users {
		result = append(result, *u)
	}
	return result, nil
}

// UpdateRole changes a user's role (mock).
func (m *MockUserRepo) UpdateRole(ctx context.Context, userID, roleID int64) error {
	if m.UpdateRoleFunc != nil {
		return m.UpdateRoleFunc(ctx, userID, roleID)
	}
	m.mu.Lock()
	defer m.mu.Unlock()

	user, ok := m.users[userID]
	if !ok {
		return fmt.Errorf("user not found")
	}
	user.RoleID = roleID
	return nil
}

// GetWithRole returns a user with role (mock).
func (m *MockUserRepo) GetWithRole(ctx context.Context, id int64) (*model.User, error) {
	if m.GetWithRoleFunc != nil {
		return m.GetWithRoleFunc(ctx, id)
	}
	m.mu.Lock()
	defer m.mu.Unlock()

	user, ok := m.users[id]
	if !ok {
		return nil, nil
	}
	return user, nil
}

// ListWithPagination returns users with pagination (mock).
func (m *MockUserRepo) ListWithPagination(ctx context.Context, emailFilter string, page, limit int) ([]model.User, int64, error) {
	if m.ListWithPaginationFunc != nil {
		return m.ListWithPaginationFunc(ctx, emailFilter, page, limit)
	}
	m.mu.Lock()
	defer m.mu.Unlock()

	result := make([]model.User, 0, len(m.users))
	for _, u := range m.users {
		result = append(result, *u)
	}
	return result, int64(len(result)), nil
}

// MarkEmailVerified marks email as verified (mock).
func (m *MockUserRepo) MarkEmailVerified(ctx context.Context, userID int64) error {
	if m.MarkEmailVerifiedFunc != nil {
		return m.MarkEmailVerifiedFunc(ctx, userID)
	}
	m.mu.Lock()
	defer m.mu.Unlock()

	user, ok := m.users[userID]
	if !ok {
		return fmt.Errorf("user not found")
	}
	user.EmailVerified = true
	now := time.Now()
	user.EmailVerifiedAt = &now
	return nil
}

// UpdatePassword updates a user's password (mock).
func (m *MockUserRepo) UpdatePassword(ctx context.Context, userID int64, hash string) error {
	if m.UpdatePasswordFunc != nil {
		return m.UpdatePasswordFunc(ctx, userID, hash)
	}
	m.mu.Lock()
	defer m.mu.Unlock()

	user, ok := m.users[userID]
	if !ok {
		return fmt.Errorf("user not found")
	}
	user.PasswordHash = hash
	return nil
}

// RecordFailedLogin increments failed attempts (mock).
func (m *MockUserRepo) RecordFailedLogin(ctx context.Context, userID int64, maxAttempts int, lockDuration time.Duration) error {
	if m.RecordFailedLoginFunc != nil {
		return m.RecordFailedLoginFunc(ctx, userID, maxAttempts, lockDuration)
	}
	m.mu.Lock()
	defer m.mu.Unlock()

	user, ok := m.users[userID]
	if !ok {
		return fmt.Errorf("user not found")
	}
	user.FailedLoginAttempts++
	if user.FailedLoginAttempts >= maxAttempts {
		lockedUntil := time.Now().Add(lockDuration)
		user.LockedUntil = &lockedUntil
	}
	return nil
}

// ResetLoginAttempts clears failed attempts (mock).
func (m *MockUserRepo) ResetLoginAttempts(ctx context.Context, userID int64) error {
	if m.ResetLoginAttemptsFunc != nil {
		return m.ResetLoginAttemptsFunc(ctx, userID)
	}
	m.mu.Lock()
	defer m.mu.Unlock()

	user, ok := m.users[userID]
	if !ok {
		return fmt.Errorf("user not found")
	}
	user.FailedLoginAttempts = 0
	user.LockedUntil = nil
	return nil
}
