package entity

import (
	"time"
)

// LeaderboardUser represents an entry on the global XP leaderboard.
type LeaderboardUser struct {
	Rank        int64
	UserID      string
	DisplayName string
	AvatarURL   string
	Xp          int64
	Level       int32
}

// UserSnapshot represents cached profile identity stored locally to avoid cross-schema JOINs.
type UserSnapshot struct {
	UserID      string
	DisplayName string
	AvatarURL   string
	UpdatedAt   time.Time
}

// NewUserSnapshot constructs a new UserSnapshot.
func NewUserSnapshot(
	userID string,
	displayName string,
	avatarURL string,
	updatedAt time.Time,
) *UserSnapshot {
	return &UserSnapshot{
		UserID:      userID,
		DisplayName: displayName,
		AvatarURL:   avatarURL,
		UpdatedAt:   updatedAt,
	}
}
