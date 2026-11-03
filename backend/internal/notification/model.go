package notification

import "time"

type NotificationResponse struct {
	ID        int64          `json:"id"`
	Type      string         `json:"type"`
	Message   string         `json:"message"`
	Metadata  map[string]any `json:"metadata"`
	ReadAt    *time.Time     `json:"read_at,omitempty"`
	CreatedAt time.Time      `json:"created_at"`
}

func (n Notification) ToResponse() NotificationResponse {
	return NotificationResponse{
		ID: n.ID, Type: n.Type, Message: n.Message, Metadata: n.Metadata,
		ReadAt: n.ReadAt, CreatedAt: n.CreatedAt,
	}
}
