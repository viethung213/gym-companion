package entity

import (
	"time"
)

// DefaultDisplayName is shown for users without a synced display name.
const DefaultDisplayName = "Gymer"

// DisplayNameOrDefault returns name, or DefaultDisplayName when name is empty.
func DisplayNameOrDefault(name string) string {
	if name == "" {
		return DefaultDisplayName
	}
	return name
}

// LeaderboardUser represents an entry on the global XP leaderboard.
type LeaderboardUser struct {
	Rank        int64
	UserID      string
	DisplayName string
	AvatarURL   string
	Xp          int64
	Level       int32
	UpdatedAt   time.Time
}

// LeaderboardScore is the ranking-relevant state of one user: lifetime XP and the time it last
// changed.
type LeaderboardScore struct {
	UserID    string
	Xp        int64
	UpdatedAt time.Time
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
