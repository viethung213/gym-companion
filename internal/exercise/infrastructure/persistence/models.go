package persistence

import (
	"database/sql"
	"time"

	"github.com/lib/pq"
)

type exerciseRecord struct {
	ID                 string         `gorm:"column:id;primaryKey"`
	Slug               string         `gorm:"column:slug"`
	Name               string         `gorm:"column:name"`
	PrimaryMuscle      string         `gorm:"column:primary_muscle"`
	SecondaryMuscles   pq.StringArray `gorm:"column:secondary_muscles;type:text[]"`
	Equipment          string         `gorm:"column:equipment"`
	Category           string         `gorm:"column:category"`
	Level              string         `gorm:"column:level"`
	Force              *string        `gorm:"column:force"`
	Mechanic           *string        `gorm:"column:mechanic"`
	Instructions       pq.StringArray `gorm:"column:instructions;type:text[]"`
	Images             pq.StringArray `gorm:"column:images;type:text[]"`
	VideoURL           *string        `gorm:"column:video_url"`
	MeasurementType    string         `gorm:"column:measurement_type"`
	DefaultRestSeconds int32          `gorm:"column:default_rest_seconds"`
	MotionSlug         *string        `gorm:"column:motion_slug"`
	HasAISupported     bool           `gorm:"column:has_ai_supported"`
	Source             string         `gorm:"column:source"`
	Status             string         `gorm:"column:status"`
	ArchivedAt         *time.Time     `gorm:"column:archived_at"`
	CreatedAt          time.Time      `gorm:"column:created_at"`
	UpdatedAt          time.Time      `gorm:"column:updated_at"`
}

func (exerciseRecord) TableName() string {
	return "exercise.exercises"
}

type outboxRecord struct {
	ID            string       `gorm:"column:id;primaryKey"`
	EventID       string       `gorm:"column:event_id;not null"`
	AggregateType string       `gorm:"column:aggregate_type"`
	AggregateID   string       `gorm:"column:aggregate_id"`
	EventType     string       `gorm:"column:event_type;not null"`
	Payload       []byte       `gorm:"column:payload;not null;type:jsonb"`
	PartitionKey  string       `gorm:"column:partition_key;not null"`
	CreatedAt     time.Time    `gorm:"column:created_at"`
	Published     bool         `gorm:"column:published;default:false"`
	PublishedAt   sql.NullTime `gorm:"column:published_at"`
	Status        string       `gorm:"column:status;default:PENDING"`
	LockedUntil   sql.NullTime `gorm:"column:locked_until"`
}

func (outboxRecord) TableName() string {
	return "exercise.outbox"
}

type outboxLogRecord struct {
	ID           string    `gorm:"column:id;primaryKey"`
	EventID      string    `gorm:"column:event_id;uniqueIndex;not null"`
	EventType    string    `gorm:"column:event_type;not null"`
	Payload      []byte    `gorm:"column:payload;not null;type:jsonb"`
	PartitionKey string    `gorm:"column:partition_key;not null"`
	ProcessedAt  time.Time `gorm:"column:processed_at;autoCreateTime"`
	Status       string    `gorm:"column:status;not null"`
	ErrorMessage string    `gorm:"column:error_message"`
}

func (outboxLogRecord) TableName() string {
	return "exercise.outbox_log"
}
