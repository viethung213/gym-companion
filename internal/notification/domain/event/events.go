package event

import (
	"time"
)

// DeviceRegisteredEvent is triggered when a user registers a new device token.
type DeviceRegisteredEvent struct {
	ID          string
	UserID      string
	DeviceToken string
	DeviceType  string
	OccurredAt  time.Time
}

// NotificationSentEvent is triggered when a push notification is dispatched.
type NotificationSentEvent struct {
	NotificationID string
	UserID         string
	Channel        string
	Title          string
	Body           string
	Data           map[string]string
	SentAt         time.Time
}

func NewDeviceRegisteredEvent(id, userID, deviceToken, deviceType string) DeviceRegisteredEvent {
	return DeviceRegisteredEvent{
		ID:          id,
		UserID:      userID,
		DeviceToken: deviceToken,
		DeviceType:  deviceType,
		OccurredAt:  time.Now().UTC(),
	}
}

func NewNotificationSentEvent(id, userID, channel, title, body string, data map[string]string) NotificationSentEvent {
	return NotificationSentEvent{
		NotificationID: id,
		UserID:         userID,
		Channel:        channel,
		Title:          title,
		Body:           body,
		Data:           data,
		SentAt:         time.Now().UTC(),
	}
}
