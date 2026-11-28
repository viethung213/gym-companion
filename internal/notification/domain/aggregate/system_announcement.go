package aggregate

import (
	"time"

	"github.com/viethung213/gym-companion/internal/notification/domain/derror"
)

// SystemAnnouncement represents a system-wide broadcast announcement.
type SystemAnnouncement struct {
	id        string
	eventID   string
	title     string
	body      string
	data      map[string]string
	createdAt time.Time
}

func NewSystemAnnouncement(id, eventID, title, body string, data map[string]string) (*SystemAnnouncement, error) {
	if title == "" {
		return nil, derror.ErrEmptyTitle
	}

	copiedData := make(map[string]string, len(data))
	for k, v := range data {
		copiedData[k] = v
	}

	return &SystemAnnouncement{
		id:        id,
		eventID:   eventID,
		title:     title,
		body:      body,
		data:      copiedData,
		createdAt: time.Now().UTC(),
	}, nil
}

func ReconstituteSystemAnnouncement(
	id, eventID, title, body string,
	data map[string]string,
	createdAt time.Time,
) *SystemAnnouncement {
	copiedData := make(map[string]string, len(data))
	for k, v := range data {
		copiedData[k] = v
	}

	return &SystemAnnouncement{
		id:        id,
		eventID:   eventID,
		title:     title,
		body:      body,
		data:      copiedData,
		createdAt: createdAt,
	}
}

func (a *SystemAnnouncement) ID() string { return a.id }

func (a *SystemAnnouncement) EventID() string { return a.eventID }

func (a *SystemAnnouncement) Title() string { return a.title }

func (a *SystemAnnouncement) Body() string { return a.body }

func (a *SystemAnnouncement) CreatedAt() time.Time { return a.createdAt }

func (a *SystemAnnouncement) Data() map[string]string {
	copied := make(map[string]string, len(a.data))
	for k, v := range a.data {
		copied[k] = v
	}
	return copied
}
