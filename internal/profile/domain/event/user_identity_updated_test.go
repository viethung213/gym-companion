//go:build unit

package event_test

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/viethung213/gym-companion/internal/profile/domain/event"
)

func TestUserIdentityUpdatedEvent(t *testing.T) {
	now := time.Now()
	tests := []struct {
		name      string
		userID    string
		fullName  string
		avatarURL string
		timeInput time.Time
	}{
		{
			name:      "Explicit time",
			userID:    "u-1",
			fullName:  "John Doe",
			avatarURL: "https://example.com/a.jpg",
			timeInput: now,
		},
		{
			name:      "Zero time defaults to current time",
			userID:    "u-2",
			fullName:  "Jane Doe",
			avatarURL: "https://example.com/b.jpg",
			timeInput: time.Time{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ev := event.NewUserIdentityUpdatedEvent(tt.userID, tt.fullName, tt.avatarURL, tt.timeInput)
			assert.Equal(t, tt.userID, ev.UserID())
			assert.Equal(t, tt.fullName, ev.FullName())
			assert.Equal(t, tt.avatarURL, ev.AvatarURL())
			if tt.timeInput.IsZero() {
				assert.False(t, ev.UpdatedAt().IsZero())
			} else {
				assert.Equal(t, tt.timeInput, ev.UpdatedAt())
			}
		})
	}
}
