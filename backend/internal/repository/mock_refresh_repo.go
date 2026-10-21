package repository

import (
	"context"
	"sync"
	"time"

	"github.com/ssr0016/pastoral-reports/backend/internal/model"
)

// MockRefreshRepo is an in-memory RefreshTokenRepository for tests. It has no
// transaction and no row lock; the real one is covered by integration tests.
type MockRefreshRepo struct {
	mu     sync.Mutex
	nextID int64
	byHash map[string]*model.RefreshToken
}

func NewMockRefreshRepo() *MockRefreshRepo {
	return &MockRefreshRepo{byHash: map[string]*model.RefreshToken{}}
}

func (m *MockRefreshRepo) Create(_ context.Context, t model.NewRefreshToken) (int64, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.nextID++
	m.byHash[t.Hash] = &model.RefreshToken{
		ID: m.nextID, UserID: t.UserID, FamilyID: t.FamilyID, ExpiresAt: t.ExpiresAt, LastUsedAt: t.Now,
	}
	return m.nextID, nil
}

func (m *MockRefreshRepo) GetForUpdate(_ context.Context, hash string) (*model.RefreshToken, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	t, ok := m.byHash[hash]
	if !ok {
		return nil, nil
	}
	c := *t
	return &c, nil
}

func (m *MockRefreshRepo) MarkReplaced(_ context.Context, id, replacedBy int64, at time.Time) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	for _, t := range m.byHash {
		if t.ID == id {
			t.RevokedAt, t.ReplacedBy = &at, &replacedBy
		}
	}
	return nil
}

func (m *MockRefreshRepo) RevokeFamily(_ context.Context, familyID string, at time.Time) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	for _, t := range m.byHash {
		if t.FamilyID == familyID && t.RevokedAt == nil {
			t.RevokedAt = &at
		}
	}
	return nil
}

// AgeRevocations moves every revocation back in time, to test the grace window.
func (m *MockRefreshRepo) AgeRevocations(by time.Duration) {
	m.mu.Lock()
	defer m.mu.Unlock()
	for _, t := range m.byHash {
		if t.RevokedAt != nil {
			at := t.RevokedAt.Add(-by)
			t.RevokedAt = &at
		}
	}
}

var _ RefreshTokenRepository = (*MockRefreshRepo)(nil)
