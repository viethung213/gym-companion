package entity

import (
	"time"
)

// StreakHistoryEntry represents a recorded active day in the user's heatmap history.
type StreakHistoryEntry struct {
	ID           string
	UserID       string
	ActivityDate string // YYYY-MM-DD
	ActivityType string // "WORKOUT", "NUTRITION"
	CreatedAt    time.Time
}

// NewStreakHistoryEntry constructs a new StreakHistoryEntry.
func NewStreakHistoryEntry(
	id string,
	userID string,
	activityDate string,
	activityType string,
	createdAt time.Time,
) *StreakHistoryEntry {
	return &StreakHistoryEntry{
		ID:           id,
		UserID:       userID,
		ActivityDate: activityDate,
		ActivityType: activityType,
		CreatedAt:    createdAt,
	}
}
