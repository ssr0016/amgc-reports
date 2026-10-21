package authimpl

import (
	"context"
	"sync"
	"time"

	"github.com/ssr0016/pastoral-reports/backend/internal/auth"
)

// MockRefreshStore is an in-memory auth.RefreshStore for tests. It has no
// transaction and no row lock; the real one is covered by integration tests.
type MockRefreshStore struct {
	mu     sync.Mutex
	nextID int64
	byHash map[string]*auth.RefreshToken
}

func NewMockRefreshStore() *MockRefreshStore {
	return &MockRefreshStore{byHash: map[string]*auth.RefreshToken{}}
}

func (m *MockRefreshStore) Create(_ context.Context, t auth.NewRefreshToken) (int64, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.nextID++
	m.byHash[t.Hash] = &auth.RefreshToken{
		ID: m.nextID, UserID: t.UserID, FamilyID: t.FamilyID, ExpiresAt: t.ExpiresAt, LastUsedAt: t.Now,
	}
	return m.nextID, nil
}

func (m *MockRefreshStore) UserIDByHash(_ context.Context, hash string) (int64, bool, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	t, ok := m.byHash[hash]
	if !ok {
		return 0, false, nil
	}
	return t.UserID, true, nil
}

func (m *MockRefreshStore) GetForUpdate(_ context.Context, hash string) (*auth.RefreshToken, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	t, ok := m.byHash[hash]
	if !ok {
		return nil, nil
	}
	c := *t
	return &c, nil
}

func (m *MockRefreshStore) MarkReplaced(_ context.Context, id, replacedBy int64, at time.Time) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	for _, t := range m.byHash {
		if t.ID == id {
			t.RevokedAt, t.ReplacedBy = &at, &replacedBy
		}
	}
	return nil
}

func (m *MockRefreshStore) RevokeFamily(_ context.Context, familyID string, at time.Time) error {
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
func (m *MockRefreshStore) AgeRevocations(by time.Duration) {
	m.mu.Lock()
	defer m.mu.Unlock()
	for _, t := range m.byHash {
		if t.RevokedAt != nil {
			at := t.RevokedAt.Add(-by)
			t.RevokedAt = &at
		}
	}
}

var _ auth.RefreshStore = (*MockRefreshStore)(nil)
