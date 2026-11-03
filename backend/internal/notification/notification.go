// Package notification owns user-scoped in-app notifications.
package notification

import (
	"context"
	"errors"
	"time"
)

var ErrNotFound = errors.New("notification not found")

type Notification struct {
	ID        int64
	UserID    int64
	Type      string
	Message   string
	Metadata  map[string]any
	ReadAt    *time.Time
	CreatedAt time.Time
}

type Store interface {
	CreateReportSubmitted(ctx context.Context, reportID int64, message string) error
	ListUnread(ctx context.Context, userID int64) ([]Notification, error)
	MarkRead(ctx context.Context, userID, id int64) error
}
