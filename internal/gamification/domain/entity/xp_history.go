package entity

import (
	"time"
)

// XpHistoryEntry represents an immutable record of earned XP.
type XpHistoryEntry struct {
	ID           string
	UserID       string
	DeltaXp      int32
	Source       string // "WORKOUT", "NUTRITION"
	SessionID    *string
	ActivityDate string // YYYY-MM-DD
	CreatedAt    time.Time
}

// NewXpHistoryEntry creates a new XP audit log entry.
func NewXpHistoryEntry(
	id string,
	userID string,
	deltaXp int32,
	source string,
	sessionID *string,
	activityDate string,
	createdAt time.Time,
) *XpHistoryEntry {
	return &XpHistoryEntry{
		ID:           id,
		UserID:       userID,
		DeltaXp:      deltaXp,
		Source:       source,
		SessionID:    sessionID,
		ActivityDate: activityDate,
		CreatedAt:    createdAt,
	}
}
