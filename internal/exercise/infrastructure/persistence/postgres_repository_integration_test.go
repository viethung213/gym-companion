//go:build integration

package persistence

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/viethung213/gym-companion/internal/exercise/application"
	"github.com/viethung213/gym-companion/internal/exercise/domain"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

func TestPostgresRepository_SaveSearchMetadataAndOutbox(t *testing.T) {
	databaseURL := os.Getenv("TEST_DATABASE_URL")
	if databaseURL == "" {
		t.Skip("TEST_DATABASE_URL is required")
	}

	db, err := gorm.Open(postgres.Open(databaseURL), &gorm.Config{})
	if err != nil {
		t.Fatalf("open database: %v", err)
	}
	sqlDB, err := db.DB()
	if err != nil {
		t.Fatalf("get database handle: %v", err)
	}
	defer sqlDB.Close()

	ctx := context.Background()
	if err := prepareExerciseSchema(ctx, db); err != nil {
		t.Fatalf("prepare schema: %v", err)
	}

	repo := NewPostgresRepository(db)
	now := time.Date(2026, 7, 11, 10, 0, 0, 0, time.UTC)
	exercise, err := domain.NewExercise(domain.Info{
		ID:                 "11111111-1111-4111-8111-111111111111",
		Name:               "Barbell Squat",
		BodyPartID:         "legs",
		EquipmentID:        "barbell",
		TargetMuscleID:     "quads",
		SecondaryMuscleIDs: []string{"glutes"},
		TagIDs:             []string{"strength"},
	}, now)
	if err != nil {
		t.Fatalf("new exercise: %v", err)
	}
	event := &domain.Event{
		ID:           "22222222-2222-4222-8222-222222222222",
		Type:         domain.EventTypeExerciseCreated,
		PartitionKey: exercise.Info().ID,
		Payload:      []byte(`{"id":"11111111-1111-4111-8111-111111111111"}`),
		CreatedAt:    now,
	}

	if err := repo.Save(ctx, exercise, event); err != nil {
		t.Fatalf("save exercise: %v", err)
	}

	loaded, err := repo.FindByID(ctx, exercise.Info().ID)
	if err != nil {
		t.Fatalf("find exercise: %v", err)
	}
	if got := loaded.Info().SecondaryMuscleIDs[0]; got != "glutes" {
		t.Fatalf("got secondary muscle %q, want glutes", got)
	}

	if err := loaded.SubmitForApproval(now); err != nil {
		t.Fatalf("submit exercise: %v", err)
	}
	if err := loaded.Approve(now); err != nil {
		t.Fatalf("approve exercise: %v", err)
	}
	if err := repo.Save(ctx, loaded, nil); err != nil {
		t.Fatalf("save active exercise: %v", err)
	}

	exercises, err := repo.SearchActive(ctx, application.SearchFilters{
		TagIDs: []string{"strength"},
	})
	if err != nil {
		t.Fatalf("search active exercises: %v", err)
	}
	if got := len(exercises); got != 1 {
		t.Fatalf("got exercises %d, want 1", got)
	}

	metadata, err := repo.GetMetadata(ctx)
	if err != nil {
		t.Fatalf("get metadata: %v", err)
	}
	if got := len(metadata.BodyParts); got != 1 {
		t.Fatalf("got body parts %d, want 1", got)
	}

	var outboxCount int
	err = db.WithContext(ctx).
		Raw(`SELECT COUNT(*) FROM exercise.outbox WHERE event_id = ?`, event.ID).
		Scan(&outboxCount).
		Error
	if err != nil {
		t.Fatalf("count outbox: %v", err)
	}
	if outboxCount != 1 {
		t.Fatalf("got outbox count %d, want 1", outboxCount)
	}
}

func prepareExerciseSchema(ctx context.Context, db *gorm.DB) error {
	statements := []string{
		`CREATE SCHEMA IF NOT EXISTS exercise`,
		`DROP TABLE IF EXISTS exercise.exercise_tags`,
		`DROP TABLE IF EXISTS exercise.exercise_secondary_muscles`,
		`DROP TABLE IF EXISTS exercise.exercises`,
		`DROP TABLE IF EXISTS exercise.tags`,
		`DROP TABLE IF EXISTS exercise.muscles`,
		`DROP TABLE IF EXISTS exercise.equipments`,
		`DROP TABLE IF EXISTS exercise.body_parts`,
		`DROP TABLE IF EXISTS exercise.outbox`,
		`CREATE TABLE exercise.body_parts (id VARCHAR(255) PRIMARY KEY, name VARCHAR(255) NOT NULL)`,
		`CREATE TABLE exercise.equipments (id VARCHAR(255) PRIMARY KEY, name VARCHAR(255) NOT NULL)`,
		`CREATE TABLE exercise.muscles (
			id VARCHAR(255) PRIMARY KEY,
			name VARCHAR(255) NOT NULL,
			body_part_id VARCHAR(255) NOT NULL REFERENCES exercise.body_parts(id)
		)`,
		`CREATE TABLE exercise.tags (id VARCHAR(255) PRIMARY KEY, name VARCHAR(255) NOT NULL)`,
		`CREATE TABLE exercise.exercises (
			id VARCHAR(255) PRIMARY KEY,
			name VARCHAR(255) NOT NULL,
			body_part_id VARCHAR(255) NOT NULL REFERENCES exercise.body_parts(id),
			equipment_id VARCHAR(255) NOT NULL REFERENCES exercise.equipments(id),
			target_muscle_id VARCHAR(255) NOT NULL REFERENCES exercise.muscles(id),
			instructions TEXT,
			thumbnail_url VARCHAR(1024),
			media_url VARCHAR(1024),
			video_url VARCHAR(1024),
			difficulty VARCHAR(50) DEFAULT 'Beginner',
			default_rest_seconds INT DEFAULT 60,
			status VARCHAR(50) DEFAULT 'DRAFT',
			archived_at TIMESTAMP WITH TIME ZONE,
			created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
			updated_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
			CONSTRAINT chk_exercises_status CHECK (
				status IN ('DRAFT', 'PENDING_APPROVAL', 'ACTIVE', 'ARCHIVED')
			)
		)`,
		`CREATE TABLE exercise.exercise_secondary_muscles (
			exercise_id VARCHAR(255) NOT NULL REFERENCES exercise.exercises(id) ON DELETE CASCADE,
			muscle_id VARCHAR(255) NOT NULL REFERENCES exercise.muscles(id) ON DELETE CASCADE,
			PRIMARY KEY (exercise_id, muscle_id)
		)`,
		`CREATE TABLE exercise.exercise_tags (
			exercise_id VARCHAR(255) NOT NULL REFERENCES exercise.exercises(id) ON DELETE CASCADE,
			tag_id VARCHAR(255) NOT NULL REFERENCES exercise.tags(id) ON DELETE CASCADE,
			PRIMARY KEY (exercise_id, tag_id)
		)`,
		`CREATE TABLE exercise.outbox (
			id UUID PRIMARY KEY,
			event_id UUID NOT NULL UNIQUE,
			event_type VARCHAR(255) NOT NULL,
			payload JSONB NOT NULL,
			partition_key VARCHAR(255) NOT NULL,
			created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP
		)`,
		`INSERT INTO exercise.body_parts (id, name) VALUES ('legs', 'Legs')`,
		`INSERT INTO exercise.equipments (id, name) VALUES ('barbell', 'Barbell')`,
		`INSERT INTO exercise.muscles (id, name, body_part_id) VALUES ('quads', 'Quads', 'legs')`,
		`INSERT INTO exercise.muscles (id, name, body_part_id) VALUES ('glutes', 'Glutes', 'legs')`,
		`INSERT INTO exercise.tags (id, name) VALUES ('strength', 'Strength')`,
	}

	for _, statement := range statements {
		if err := db.WithContext(ctx).Exec(statement).Error; err != nil {
			return err
		}
	}

	return nil
}
