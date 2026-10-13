package event

import (
	"time"
)

// Event is the marker interface for domain events.
type Event interface {
	EventName() string
	PartitionKey() string
}

// XpEarned represents the domain event when a user gains XP.
type XpEarned struct {
	UserID   string
	DeltaXp  int32
	NewXp    int64
	NewLevel int32
	Source   string
	EarnedAt time.Time
}

func (XpEarned) EventName() string {
	return "contracts.supporting.gamification.v1.xpEarned"
}

func (e XpEarned) PartitionKey() string {
	return e.UserID
}

// UserLeveledUp represents the domain event when a user levels up.
type UserLeveledUp struct {
	UserID      string
	OldLevel    int32
	NewLevel    int32
	TotalXp     int64
	LeveledUpAt time.Time
}

func (UserLeveledUp) EventName() string {
	return "contracts.supporting.gamification.v1.userLeveledUp"
}

func (e UserLeveledUp) PartitionKey() string {
	return e.UserID
}

// StreakAdvanced represents the domain event when a user's streak advances.
type StreakAdvanced struct {
	UserID        string
	NewStreak     int32
	LongestStreak int32
	ActivityDate  string
	ActivityType  string
	AdvancedAt    time.Time
}

func (StreakAdvanced) EventName() string {
	return "contracts.supporting.gamification.v1.streakAdvanced"
}

func (e StreakAdvanced) PartitionKey() string {
	return e.UserID
}

// StreakReset represents the domain event when a user's streak resets.
type StreakReset struct {
	UserID         string
	PreviousStreak int32
	ResetDate      string
	ResetAt        time.Time
}

func (StreakReset) EventName() string {
	return "contracts.supporting.gamification.v1.streakReset"
}

func (e StreakReset) PartitionKey() string {
	return e.UserID
}

// StreakMilestoneReached represents the domain event when a user reaches a streak milestone.
type StreakMilestoneReached struct {
	UserID        string
	MilestoneDays int32
	ReachedAt     time.Time
}

func (StreakMilestoneReached) EventName() string {
	return "contracts.supporting.gamification.v1.streakMilestoneReached"
}

func (e StreakMilestoneReached) PartitionKey() string {
	return e.UserID
}

// BadgeUnlocked represents the domain event when a user unlocks a badge.
type BadgeUnlocked struct {
	UserID     string
	BadgeCode  string
	BadgeName  string
	Category   string
	UnlockedAt time.Time
}

func (BadgeUnlocked) EventName() string {
	return "contracts.supporting.gamification.v1.badgeUnlocked"
}

func (e BadgeUnlocked) PartitionKey() string {
	return e.UserID
}
