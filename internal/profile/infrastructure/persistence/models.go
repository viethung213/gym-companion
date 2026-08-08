package persistence

import (
	"database/sql"
	"time"
)

type UserProfileModel struct {
	UserID                string     `gorm:"primaryKey;column:user_id"`
	FullName              string     `gorm:"column:full_name"`
	AvatarURL             string     `gorm:"column:avatar_url"`
	DateOfBirth           *time.Time `gorm:"column:date_of_birth"`
	Gender                string     `gorm:"column:gender"`
	ExperienceLevel       string     `gorm:"column:experience_level"`
	Goals                 []byte     `gorm:"column:goals;type:jsonb"`
	PreferredWorkoutTimes []byte     `gorm:"column:preferred_workout_times;type:jsonb"`
	AvailableEquipment    []byte     `gorm:"column:available_equipment;type:jsonb"`
	PreferredMuscleGroups []byte     `gorm:"column:preferred_muscle_groups;type:jsonb"`
	CoachStyle            string     `gorm:"column:coach_style"`
	TargetWeightKg        float64    `gorm:"column:target_weight_kg"`
	TargetBodyFatPercent  float64    `gorm:"column:target_body_fat_percent"`
	CompletionRate        float64    `gorm:"column:completion_rate"`
	AICoachActivated      bool       `gorm:"column:ai_coach_activated"`
	CreatedAt             time.Time  `gorm:"column:created_at"`
	UpdatedAt             time.Time  `gorm:"column:updated_at"`
}

func (UserProfileModel) TableName() string {
	return "profile.users"
}

type BodyMetricModel struct {
	ID               string    `gorm:"primaryKey;column:id"`
	UserID           string    `gorm:"column:user_id"`
	WeightKg         float64   `gorm:"column:weight_kg"`
	HeightCm         float64   `gorm:"column:height_cm"`
	BodyFatPercent   float64   `gorm:"column:body_fat_percent"`
	ProgressPhotoURL string    `gorm:"column:progress_photo_url"`
	LoggedAt         time.Time `gorm:"column:logged_at"`
}

func (BodyMetricModel) TableName() string {
	return "profile.body_metrics"
}

type InjuryModel struct {
	ID          string     `gorm:"primaryKey;column:id"`
	UserID      string     `gorm:"column:user_id"`
	MuscleGroup string     `gorm:"column:muscle_group"`
	Severity    string     `gorm:"column:severity"`
	Notes       string     `gorm:"column:notes"`
	ReportedAt  time.Time  `gorm:"column:reported_at"`
	IsRecovered bool       `gorm:"column:is_recovered"`
	RecoveredAt *time.Time `gorm:"column:recovered_at"`
	CreatedAt   time.Time  `gorm:"column:created_at"`
	UpdatedAt   time.Time  `gorm:"column:updated_at"`
}

func (InjuryModel) TableName() string {
	return "profile.injuries"
}

type OutboxModel struct {
	ID           string       `gorm:"primaryKey;column:id"`
	EventID      string       `gorm:"column:event_id;uniqueIndex;not null"`
	EventType    string       `gorm:"column:event_type;not null"`
	Payload      []byte       `gorm:"column:payload;not null;type:jsonb"`
	PartitionKey string       `gorm:"column:partition_key;not null"`
	CreatedAt    time.Time    `gorm:"column:created_at"`
	Published    bool         `gorm:"column:published"`
	PublishedAt  *time.Time   `gorm:"column:published_at"`
	Status       string       `gorm:"column:status;default:PENDING"`
	LockedUntil  sql.NullTime `gorm:"column:locked_until"`
}

func (OutboxModel) TableName() string {
	return "profile.outbox"
}

type OutboxLogModel struct {
	ID           string    `gorm:"primaryKey;column:id"`
	EventID      string    `gorm:"column:event_id;index;not null"`
	EventType    string    `gorm:"column:event_type;not null"`
	Payload      []byte    `gorm:"column:payload;not null;type:jsonb"`
	PartitionKey string    `gorm:"column:partition_key;not null"`
	ProcessedAt  time.Time `gorm:"column:processed_at"`
	Status       string    `gorm:"column:status;not null"`
	ErrorMessage string    `gorm:"column:error_message"`
}

func (OutboxLogModel) TableName() string {
	return "profile.outbox_log"
}
