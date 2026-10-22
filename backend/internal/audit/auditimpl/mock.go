package auditimpl

import (
	"context"
	"sync"

	"github.com/ssr0016/pastoral-reports/backend/internal/audit"
)

// MockStore records audit rows in memory for testing.
type MockStore struct {
	mu      sync.Mutex
	entries []audit.Log

	// Override function for testing specific behaviors
	InsertFunc func(ctx context.Context, e audit.Log) error
}

func NewMockStore() *MockStore { return &MockStore{} }

// Insert applies the same details allow-list as the real store, so a service
// test fails when a writer adds a key the store would refuse.
func (m *MockStore) Insert(ctx context.Context, e audit.Log) error {
	if err := e.CheckDetails(); err != nil {
		return err
	}
	if m.InsertFunc != nil {
		return m.InsertFunc(ctx, e)
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	m.entries = append(m.entries, e)
	return nil
}

// Entries returns a copy of the recorded rows.
func (m *MockStore) Entries() []audit.Log {
	m.mu.Lock()
	defer m.mu.Unlock()
	return append([]audit.Log(nil), m.entries...)
}

var _ audit.Store = (*MockStore)(nil)
