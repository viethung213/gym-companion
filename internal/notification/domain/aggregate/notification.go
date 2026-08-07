package aggregate

import (
	"time"

	"github.com/viethung213/gym-companion/internal/notification/domain/derror"
)

// InAppNotification represents an in-app notification history record.
type InAppNotification struct {
	id        string
	userID    string
	title     string
	body      string
	data      map[string]string
	isRead    bool
	createdAt time.Time
}

func NewInAppNotification(id, userID, title, body string, data map[string]string) (*InAppNotification, error) {
	if userID == "" {
		return nil, derror.ErrEmptyUserID
	}
	if title == "" {
		return nil, derror.ErrEmptyTitle
	}

	copiedData := make(map[string]string, len(data))
	for k, v := range data {
		copiedData[k] = v
	}

	return &InAppNotification{
		id:        id,
		userID:    userID,
		title:     title,
		body:      body,
		data:      copiedData,
		isRead:    false,
		createdAt: time.Now().UTC(),
	}, nil
}

func ReconstituteInAppNotification(
	id, userID, title, body string,
	data map[string]string,
	isRead bool,
	createdAt time.Time,
) *InAppNotification {
	copiedData := make(map[string]string, len(data))
	for k, v := range data {
		copiedData[k] = v
	}

	return &InAppNotification{
		id:        id,
		userID:    userID,
		title:     title,
		body:      body,
		data:      copiedData,
		isRead:    isRead,
		createdAt: createdAt,
	}
}

func (n *InAppNotification) ID() string           { return n.id }
func (n *InAppNotification) UserID() string       { return n.userID }
func (n *InAppNotification) Title() string        { return n.title }
func (n *InAppNotification) Body() string         { return n.body }
func (n *InAppNotification) IsRead() bool         { return n.isRead }
func (n *InAppNotification) CreatedAt() time.Time { return n.createdAt }

func (n *InAppNotification) Data() map[string]string {
	copiedData := make(map[string]string, len(n.data))
	for k, v := range n.data {
		copiedData[k] = v
	}
	return copiedData
}

func (n *InAppNotification) MarkAsRead() {
	n.isRead = true
}
