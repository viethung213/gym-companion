package persistence

import (
	"database/sql"
	"time"
)

type roadmapRecord struct {
	ID        string    `gorm:"column:id;primaryKey"`
	UserID    string    `gorm:"column:user_id;not null"`
	Status    string    `gorm:"column:status;not null;default:ACTIVE"`
	StartDate time.Time `gorm:"column:start_date;not null"`
	EndDate   time.Time `gorm:"column:end_date;not null"`
	CreatedAt time.Time `gorm:"column:created_at"`
	UpdatedAt time.Time `gorm:"column:updated_at"`
}

func (roadmapRecord) TableName() string {
	return "coaching.workout_roadmaps"
}

type weeklyScheduleRecord struct {
	ID              string    `gorm:"column:id;primaryKey"`
	RoadmapID       string    `gorm:"column:roadmap_id;not null"`
	UserID          string    `gorm:"column:user_id;not null"`
	WeekNumber      int       `gorm:"column:week_number;not null"`
	StartDate       time.Time `gorm:"column:start_date;not null"`
	EndDate         time.Time `gorm:"column:end_date;not null"`
	MuscleSplitType string    `gorm:"column:muscle_split_type;not null"`
	ScheduleDays    []byte    `gorm:"column:schedule_days;not null;type:jsonb"`
	CreatedAt       time.Time `gorm:"column:created_at"`
	UpdatedAt       time.Time `gorm:"column:updated_at"`
}

func (weeklyScheduleRecord) TableName() string {
	return "coaching.weekly_schedules"
}

type dailyWorkoutPlanRecord struct {
	ID                    string    `gorm:"column:id;primaryKey"`
	WeeklyScheduleID      string    `gorm:"column:weekly_schedule_id;not null"`
	RoadmapID             string    `gorm:"column:roadmap_id;not null"`
	UserID                string    `gorm:"column:user_id;not null"`
	ScheduledDate         time.Time `gorm:"column:scheduled_date;not null"`
	Status                string    `gorm:"column:status;not null;default:DRAFT"`
	WorkoutPrescription   []byte    `gorm:"column:workout_prescription;not null;type:jsonb"`
	ReasoningExplanation  *string   `gorm:"column:reasoning_explanation"`
	AdjustmentExplanation *string   `gorm:"column:adjustment_explanation"`
	CreatedAt             time.Time `gorm:"column:created_at"`
	UpdatedAt             time.Time `gorm:"column:updated_at"`
}

func (dailyWorkoutPlanRecord) TableName() string {
	return "coaching.daily_workout_plans"
}

type outboxRecord struct {
	ID           string       `gorm:"column:id;primaryKey"`
	EventID      string       `gorm:"column:event_id;not null"`
	EventType    string       `gorm:"column:event_type;not null"`
	Payload      []byte       `gorm:"column:payload;not null;type:jsonb"`
	PartitionKey string       `gorm:"column:partition_key;not null"`
	CreatedAt    time.Time    `gorm:"column:created_at"`
	Published    bool         `gorm:"column:published;default:false"`
	PublishedAt  sql.NullTime `gorm:"column:published_at"`
}

func (outboxRecord) TableName() string {
	return "coaching.outbox"
}

type outboxLogRecord struct {
	ID           string    `gorm:"column:id;primaryKey"`
	EventID      string    `gorm:"column:event_id;not null"`
	EventType    string    `gorm:"column:event_type;not null"`
	Payload      []byte    `gorm:"column:payload;not null;type:jsonb"`
	PartitionKey string    `gorm:"column:partition_key;not null"`
	ProcessedAt  time.Time `gorm:"column:processed_at"`
	Status       string    `gorm:"column:status;not null"`
	ErrorMessage *string   `gorm:"column:error_message"`
}

func (outboxLogRecord) TableName() string {
	return "coaching.outbox_log"
}
