package event

import (
	"time"
)

type Event interface {
	EventName() string
	PartitionKey() string
}

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

//nolint:gocritic // hugeParam: domain event PartitionKey value receiver
func (e XpEarned) PartitionKey() string {
	return e.UserID
}

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

//nolint:gocritic // hugeParam: domain event PartitionKey value receiver
func (e UserLeveledUp) PartitionKey() string {
	return e.UserID
}

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

//nolint:gocritic // hugeParam: domain event PartitionKey value receiver
func (e StreakAdvanced) PartitionKey() string {
	return e.UserID
}

type StreakReset struct {
	UserID         string
	PreviousStreak int32
	ResetDate      string
	ResetAt        time.Time
}

func (StreakReset) EventName() string {
	return "contracts.supporting.gamification.v1.streakReset"
}

//nolint:gocritic // hugeParam: domain event PartitionKey value receiver
func (e StreakReset) PartitionKey() string {
	return e.UserID
}

type StreakMilestoneReached struct {
	UserID        string
	MilestoneDays int32
	ReachedAt     time.Time
}

func (StreakMilestoneReached) EventName() string {
	return "contracts.supporting.gamification.v1.streakMilestoneReached"
}

//nolint:gocritic // hugeParam: domain event PartitionKey value receiver
func (e StreakMilestoneReached) PartitionKey() string {
	return e.UserID
}

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

//nolint:gocritic // hugeParam: domain event PartitionKey value receiver
func (e BadgeUnlocked) PartitionKey() string {
	return e.UserID
}

type CoinsEarned struct {
	UserID       string
	Amount       int64
	BalanceAfter int64
	Reason       string
	ReferenceID  string
	EarnedAt     time.Time
}

func (CoinsEarned) EventName() string {
	return "contracts.supporting.gamification.v1.coinsEarned"
}

//nolint:gocritic // hugeParam: domain event PartitionKey value receiver
func (e CoinsEarned) PartitionKey() string {
	return e.UserID
}

type CoinsSpent struct {
	UserID       string
	Amount       int64
	BalanceAfter int64
	Reason       string
	ReferenceID  string
	SpentAt      time.Time
}

func (CoinsSpent) EventName() string {
	return "contracts.supporting.gamification.v1.coinsSpent"
}

//nolint:gocritic // hugeParam: domain event PartitionKey value receiver
func (e CoinsSpent) PartitionKey() string {
	return e.UserID
}

type WorkoutActivityIngested struct {
	UserID           string
	SessionID        string
	ActivityDate     string
	TotalSets        int32
	TotalVolumeKg    float64
	VolumeRatio      *float64
	AverageFormScore *float32
	HasNewPR         bool
	CompletedAt      time.Time
}

func (WorkoutActivityIngested) EventName() string {
	return "internal.gamification.workoutActivityIngested"
}

//nolint:gocritic // hugeParam: domain event PartitionKey value receiver
func (e WorkoutActivityIngested) PartitionKey() string {
	return e.UserID
}

type NutritionActivityIngested struct {
	UserID       string
	ActivityDate string
	RecordedAt   time.Time
}

func (NutritionActivityIngested) EventName() string {
	return "internal.gamification.nutritionActivityIngested"
}

//nolint:gocritic // hugeParam: domain event PartitionKey value receiver
func (e NutritionActivityIngested) PartitionKey() string {
	return e.UserID
}

type PersonalRecordIngested struct {
	UserID       string
	ExerciseID   string
	Weight       float64
	Reps         int32
	ActivityDate string
	AchievedAt   time.Time
}

func (PersonalRecordIngested) EventName() string {
	return "internal.gamification.personalRecordIngested"
}

//nolint:gocritic // hugeParam: domain event PartitionKey value receiver
func (e PersonalRecordIngested) PartitionKey() string {
	return e.UserID
}
