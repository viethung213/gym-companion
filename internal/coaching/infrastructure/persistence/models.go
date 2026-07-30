package persistence

import (
	"database/sql"
	"time"
)

// roadmapRecord maps coaching.roadmaps.
type roadmapRecord struct {
	RoadmapID string    `gorm:"column:roadmap_id;primaryKey"`
	UserID    string    `gorm:"column:user_id;not null"`
	Status    string    `gorm:"column:status;not null"`
	StartDate time.Time `gorm:"column:start_date;not null;type:date"`
	EndDate   time.Time `gorm:"column:end_date;not null;type:date"`
	CreatedAt time.Time `gorm:"column:created_at"`
	UpdatedAt time.Time `gorm:"column:updated_at"`
}

func (roadmapRecord) TableName() string { return "coaching.roadmaps" }

// weekPlanRecord maps coaching.week_plans.
type weekPlanRecord struct {
	WeekPlanID      string    `gorm:"column:week_plan_id;primaryKey"`
	RoadmapID       string    `gorm:"column:roadmap_id;not null"`
	UserID          string    `gorm:"column:user_id;not null"`
	WeekNumber      int16     `gorm:"column:week_number;not null"`
	Phase           string    `gorm:"column:phase;not null"`
	TargetRPE       float32   `gorm:"column:target_rpe;not null"`
	StartDate       time.Time `gorm:"column:start_date;not null;type:date"`
	EndDate         time.Time `gorm:"column:end_date;not null;type:date"`
	MuscleSplitType string    `gorm:"column:muscle_split_type"`
	CreatedAt       time.Time `gorm:"column:created_at"`
}

func (weekPlanRecord) TableName() string { return "coaching.week_plans" }

// dayPlanRecord maps coaching.day_plans.
type dayPlanRecord struct {
	DayPlanID     string    `gorm:"column:day_plan_id;primaryKey"`
	WeekPlanID    string    `gorm:"column:week_plan_id;not null"`
	RoadmapID     string    `gorm:"column:roadmap_id;not null"`
	UserID        string    `gorm:"column:user_id;not null"`
	ScheduledDate time.Time `gorm:"column:scheduled_date;not null;type:date"`
	CreatedAt     time.Time `gorm:"column:created_at"`
}

func (dayPlanRecord) TableName() string { return "coaching.day_plans" }

// sessionPlanRecord maps coaching.session_plans. Prescription and
// TargetMuscleGroups are JSONB blobs.
type sessionPlanRecord struct {
	SessionPlanID      string          `gorm:"column:session_plan_id;primaryKey"`
	DayPlanID          string          `gorm:"column:day_plan_id;not null"`
	WeekPlanID         string          `gorm:"column:week_plan_id;not null"`
	RoadmapID          string          `gorm:"column:roadmap_id;not null"`
	UserID             string          `gorm:"column:user_id;not null"`
	ScheduledDate      time.Time       `gorm:"column:scheduled_date;not null;type:date"`
	SlotTime           string          `gorm:"column:slot_time"`
	Status             string          `gorm:"column:status;not null"`
	Source             string          `gorm:"column:source;not null"`
	TargetMuscleGroups []byte          `gorm:"column:target_muscle_groups;type:jsonb"`
	Prescription       []byte          `gorm:"column:prescription;type:jsonb"`
	Reasoning          string          `gorm:"column:reasoning"`
	GeneratedAt        time.Time       `gorm:"column:generated_at"`
	CompletedAt        sql.NullTime    `gorm:"column:completed_at"`
	SessionSCR         sql.NullFloat64 `gorm:"column:session_scr"`
	SessionDeltaRPE    sql.NullFloat64 `gorm:"column:session_delta_rpe"`
}

func (sessionPlanRecord) TableName() string { return "coaching.session_plans" }

// outboxRecord maps coaching.outbox.
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

func (outboxRecord) TableName() string { return "coaching.outbox" }

// outboxLogRecord maps coaching.outbox_log (D9 consumer idempotency).
type outboxLogRecord struct {
	ID           string    `gorm:"column:id;primaryKey"`
	EventID      string    `gorm:"column:event_id;not null"`
	EventType    string    `gorm:"column:event_type;not null"`
	Payload      []byte    `gorm:"column:payload;not null;type:jsonb"`
	PartitionKey string    `gorm:"column:partition_key;not null"`
	ProcessedAt  time.Time `gorm:"column:processed_at"`
	Status       string    `gorm:"column:status;not null"`
	ErrorMessage string    `gorm:"column:error_message"`
}

func (outboxLogRecord) TableName() string { return "coaching.outbox_log" }
