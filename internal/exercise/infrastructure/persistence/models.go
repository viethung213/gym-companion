package persistence

import (
	"database/sql"
	"time"
)

type exerciseRecord struct {
	ID                 string     `gorm:"column:id;primaryKey"`
	Name               string     `gorm:"column:name"`
	BodyPartID         string     `gorm:"column:body_part_id"`
	EquipmentID        string     `gorm:"column:equipment_id"`
	TargetMuscleID     string     `gorm:"column:target_muscle_id"`
	Instructions       string     `gorm:"column:instructions"`
	ThumbnailURL       *string    `gorm:"column:thumbnail_url"`
	MediaURL           *string    `gorm:"column:media_url"`
	VideoURL           *string    `gorm:"column:video_url"`
	Difficulty         string     `gorm:"column:difficulty"`
	DefaultRestSeconds int32      `gorm:"column:default_rest_seconds"`
	Status             string     `gorm:"column:status"`
	ArchivedAt         *time.Time `gorm:"column:archived_at"`
	CreatedAt          time.Time  `gorm:"column:created_at"`
	UpdatedAt          time.Time  `gorm:"column:updated_at"`
}

func (exerciseRecord) TableName() string {
	return "exercise.exercises"
}

type exerciseSecondaryMuscleRecord struct {
	ExerciseID string `gorm:"column:exercise_id;primaryKey"`
	MuscleID   string `gorm:"column:muscle_id;primaryKey"`
}

func (exerciseSecondaryMuscleRecord) TableName() string {
	return "exercise.exercise_secondary_muscles"
}

type exerciseTagRecord struct {
	ExerciseID string `gorm:"column:exercise_id;primaryKey"`
	TagID      string `gorm:"column:tag_id;primaryKey"`
}

func (exerciseTagRecord) TableName() string {
	return "exercise.exercise_tags"
}

type bodyPartRecord struct {
	ID   string `gorm:"column:id;primaryKey"`
	Name string `gorm:"column:name"`
}

func (bodyPartRecord) TableName() string {
	return "exercise.body_parts"
}

type equipmentRecord struct {
	ID   string `gorm:"column:id;primaryKey"`
	Name string `gorm:"column:name"`
}

func (equipmentRecord) TableName() string {
	return "exercise.equipments"
}

type muscleRecord struct {
	ID         string `gorm:"column:id;primaryKey"`
	Name       string `gorm:"column:name"`
	BodyPartID string `gorm:"column:body_part_id"`
}

func (muscleRecord) TableName() string {
	return "exercise.muscles"
}

type tagRecord struct {
	ID   string `gorm:"column:id;primaryKey"`
	Name string `gorm:"column:name"`
}

func (tagRecord) TableName() string {
	return "exercise.tags"
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
	return "exercise.outbox"
}
