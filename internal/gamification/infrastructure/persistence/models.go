package persistence

import (
	"time"
)

// OutboxModel maps to gamification.outbox table.
type OutboxModel struct {
	ID            string     `gorm:"column:id;primaryKey"`
	EventID       string     `gorm:"column:event_id;uniqueIndex"`
	AggregateType string     `gorm:"column:aggregate_type"`
	AggregateID   string     `gorm:"column:aggregate_id"`
	EventType     string     `gorm:"column:event_type"`
	Payload       []byte     `gorm:"column:payload;type:jsonb"`
	PartitionKey  string     `gorm:"column:partition_key"`
	Published     bool       `gorm:"column:published"`
	PublishedAt   *time.Time `gorm:"column:published_at"`
	CreatedAt     time.Time  `gorm:"column:created_at"`
	Status        string     `gorm:"column:status"`
	RetryCount    int        `gorm:"column:retry_count"`
	MaxRetries    int        `gorm:"column:max_retries"`
	LastError     *string    `gorm:"column:last_error"`
	LockedUntil   *time.Time `gorm:"column:locked_until"`
}

// TableName returns the table name for OutboxModel.
func (OutboxModel) TableName() string {
	return "gamification.outbox"
}

// OutboxLogModel maps to gamification.outbox_log table.
type OutboxLogModel struct {
	ID           string    `gorm:"column:id;primaryKey"`
	EventID      string    `gorm:"column:event_id;uniqueIndex"`
	EventType    string    `gorm:"column:event_type"`
	Payload      []byte    `gorm:"column:payload;type:jsonb"`
	PartitionKey string    `gorm:"column:partition_key"`
	ProcessedAt  time.Time `gorm:"column:processed_at"`
	Status       string    `gorm:"column:status"`
	ErrorMessage string    `gorm:"column:error_message"`
}

// TableName returns the table name for OutboxLogModel.
func (OutboxLogModel) TableName() string {
	return "gamification.outbox_log"
}

// UserXpModel maps to gamification.user_xp table.
type UserXpModel struct {
	UserID                  string     `gorm:"column:user_id;primaryKey"`
	Xp                      int64      `gorm:"column:xp"`
	Level                   int32      `gorm:"column:level"`
	LastNutritionRewardDate *time.Time `gorm:"column:last_nutrition_reward_date;type:date"`
	CreatedAt               time.Time  `gorm:"column:created_at"`
	UpdatedAt               time.Time  `gorm:"column:updated_at"`
}

// TableName returns the table name for UserXpModel.
func (UserXpModel) TableName() string {
	return "gamification.user_xp"
}

// XpHistoryModel maps to gamification.xp_history table.
type XpHistoryModel struct {
	ID           string    `gorm:"column:id;primaryKey"`
	UserID       string    `gorm:"column:user_id"`
	DeltaXp      int32     `gorm:"column:amount"`
	Source       string    `gorm:"column:source"`
	SessionID    *string   `gorm:"column:session_id"`
	ActivityDate time.Time `gorm:"column:activity_date;type:date"`
	CreatedAt    time.Time `gorm:"column:created_at"`
}

// TableName returns the table name for XpHistoryModel.
func (XpHistoryModel) TableName() string {
	return "gamification.xp_history"
}

// UserStreakModel maps to gamification.user_streak table.
type UserStreakModel struct {
	UserID           string     `gorm:"column:user_id;primaryKey"`
	CurrentStreak    int32      `gorm:"column:current_streak"`
	LongestStreak    int32      `gorm:"column:longest_streak"`
	LastActivityDate *time.Time `gorm:"column:last_active_date;type:date"`
	CreatedAt        time.Time  `gorm:"column:created_at"`
	UpdatedAt        time.Time  `gorm:"column:updated_at"`
}

// TableName returns the table name for UserStreakModel.
func (UserStreakModel) TableName() string {
	return "gamification.user_streak"
}

// StreakHistoryModel maps to gamification.streak_history table.
type StreakHistoryModel struct {
	ID           string    `gorm:"column:id;primaryKey"`
	UserID       string    `gorm:"column:user_id"`
	ActivityDate time.Time `gorm:"column:activity_date;type:date"`
	ActivityType string    `gorm:"column:activity_type"`
	CreatedAt    time.Time `gorm:"column:created_at"`
}

// TableName returns the table name for StreakHistoryModel.
func (StreakHistoryModel) TableName() string {
	return "gamification.streak_history"
}

// UserSnapshotModel maps to gamification.user_snapshots table.
type UserSnapshotModel struct {
	UserID      string    `gorm:"column:user_id;primaryKey"`
	DisplayName string    `gorm:"column:display_name"`
	AvatarURL   string    `gorm:"column:avatar_url"`
	UpdatedAt   time.Time `gorm:"column:updated_at"`
}

// TableName returns the table name for UserSnapshotModel.
func (UserSnapshotModel) TableName() string {
	return "gamification.user_snapshots"
}

// UserBadgeModel maps to gamification.user_badges table.
type UserBadgeModel struct {
	ID         string    `gorm:"column:id;primaryKey"`
	UserID     string    `gorm:"column:user_id"`
	BadgeCode  string    `gorm:"column:badge_code"`
	UnlockedAt time.Time `gorm:"column:unlocked_at"`
	Metadata   []byte    `gorm:"column:metadata;type:jsonb"`
}

// TableName returns the table name for UserBadgeModel.
func (UserBadgeModel) TableName() string {
	return "gamification.user_badges"
}
