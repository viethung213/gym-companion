package entity

import (
	"strings"
	"time"

	"github.com/viethung213/gym-companion/internal/solo_match/domain/derror"
)

type UserSnapshot struct {
	userID      string
	displayName string
	avatarURL   string
	syncedAt    time.Time
}

func NewUserSnapshot(userID, displayName, avatarURL string, syncedAt time.Time) (*UserSnapshot, error) {
	trimmedID := strings.TrimSpace(userID)
	if trimmedID == "" {
		return nil, derror.ErrEmptyUserID
	}
	trimmedName := strings.TrimSpace(displayName)
	if trimmedName == "" {
		trimmedName = "Gym Athlete"
	}
	if syncedAt.IsZero() {
		syncedAt = time.Now().UTC()
	}

	return &UserSnapshot{
		userID:      trimmedID,
		displayName: trimmedName,
		avatarURL:   strings.TrimSpace(avatarURL),
		syncedAt:    syncedAt,
	}, nil
}

func (u *UserSnapshot) UserID() string {
	return u.userID
}

func (u *UserSnapshot) DisplayName() string {
	return u.displayName
}

func (u *UserSnapshot) AvatarURL() string {
	return u.avatarURL
}

func (u *UserSnapshot) SyncedAt() time.Time {
	return u.syncedAt
}

func (u *UserSnapshot) Update(displayName, avatarURL string, syncedAt time.Time) {
	trimmedName := strings.TrimSpace(displayName)
	if trimmedName != "" {
		u.displayName = trimmedName
	}
	u.avatarURL = strings.TrimSpace(avatarURL)
	if syncedAt.IsZero() {
		syncedAt = time.Now().UTC()
	}
	u.syncedAt = syncedAt
}
