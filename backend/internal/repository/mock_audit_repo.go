package repository

import (
	"context"
	"sync"

	"github.com/ssr0016/pastoral-reports/backend/internal/model"
)

// MockAuditRepo records audit rows in memory for testing.
type MockAuditRepo struct {
	mu      sync.Mutex
	entries []model.AuditLog

	// Override function for testing specific behaviors
	InsertFunc func(ctx context.Context, e model.AuditLog) error
}

func NewMockAuditRepo() *MockAuditRepo { return &MockAuditRepo{} }

func (m *MockAuditRepo) Insert(ctx context.Context, e model.AuditLog) error {
	if m.InsertFunc != nil {
		return m.InsertFunc(ctx, e)
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	m.entries = append(m.entries, e)
	return nil
}

// Entries returns a copy of the recorded rows.
func (m *MockAuditRepo) Entries() []model.AuditLog {
	m.mu.Lock()
	defer m.mu.Unlock()
	return append([]model.AuditLog(nil), m.entries...)
}
