package seeder

import (
	"context"
	"database/sql"
	_ "embed"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"time"

	exercisemsg "github.com/viethung213/gym-companion/internal/gen/go/contracts/supporting/exercise/v1/message"
	"github.com/viethung213/gym-companion/internal/shared/middleware"
)

// upstreamExercisesJSON is free-exercise-db dist/exercises.json at the commit
// pinned in data/upstream/free-exercise-db/UPSTREAM.md.
//
//go:embed data/upstream/free-exercise-db/exercises.json
var upstreamExercisesJSON []byte

// aiMotionMapJSON maps each AI motion_slug to the source exercise ids it
// covers (docs/05 §8.8).
//
//go:embed data/ai_motion_map.json
var aiMotionMapJSON []byte

const (
	stretchingRestSeconds = 15
	compoundRestSeconds   = 90
	defaultRestSeconds    = 60
)

// upstreamExercise is one free-exercise-db record; nullable fields are pointers.
type upstreamExercise struct {
	ID               string   `json:"id"`
	Name             string   `json:"name"`
	Force            *string  `json:"force"`
	Level            string   `json:"level"`
	Mechanic         *string  `json:"mechanic"`
	Equipment        *string  `json:"equipment"`
	PrimaryMuscles   []string `json:"primaryMuscles"`
	SecondaryMuscles []string `json:"secondaryMuscles"`
	Instructions     []string `json:"instructions"`
	Category         string   `json:"category"`
	Images           []string `json:"images"`
}

// ExerciseRegistrar creates catalog exercises through the Exercise service, so
// seeding keeps the aggregate invariants and the outbox events.
type ExerciseRegistrar interface {
	CreateExercise(ctx context.Context, req *exercisemsg.CreateExerciseRequest) (*exercisemsg.CreateExerciseResponse, error)
}

// SeedExercises imports the pinned free-exercise-db catalog, skipping
// exercises whose slug already exists. Exercises with images are created
// ACTIVE, the others DRAFT.
func SeedExercises(ctx context.Context, db *sql.DB, exerciseService ExerciseRegistrar) error {
	if exerciseService == nil {
		return errors.New("exerciseService cannot be nil")
	}

	var records []upstreamExercise
	if err := json.Unmarshal(upstreamExercisesJSON, &records); err != nil {
		return fmt.Errorf("unmarshal upstream exercises: %w", err)
	}
	motionSlugs, err := loadAIMotionMap()
	if err != nil {
		return err
	}
	existing, err := existingExerciseSlugs(ctx, db)
	if err != nil {
		return err
	}

	adminCtx := context.WithValue(ctx, middleware.UserIDKey, "system-seeder-admin")
	adminCtx = context.WithValue(adminCtx, middleware.UserRoleKey, "Admin")

	slog.InfoContext(ctx, "Starting Exercise catalog seeding",
		slog.Int("total_records", len(records)),
		slog.Int("already_existing", len(existing)),
	)

	var failures []error
	createdCount := 0
	for i := range records {
		if err := ctx.Err(); err != nil {
			return err
		}

		record := &records[i]
		if existing[record.ID] {
			continue
		}

		req, err := newCreateExerciseRequest(record, motionSlugs[record.ID])
		if err != nil {
			return err
		}
		if _, err := exerciseService.CreateExercise(adminCtx, req); err != nil {
			failures = append(failures, fmt.Errorf("create exercise %s: %w", record.ID, err))
			continue
		}

		existing[record.ID] = true
		createdCount++

		// Yield briefly every 25 inserts so the seeder does not starve the connection pool.
		if createdCount%25 == 0 {
			time.Sleep(5 * time.Millisecond)
		}
	}

	slog.InfoContext(ctx, "Completed Exercise catalog seeding",
		slog.Int("new_exercises_created", createdCount),
		slog.Int("failed", len(failures)),
	)

	return errors.Join(failures...)
}

// loadAIMotionMap returns source exercise id -> motion_slug.
func loadAIMotionMap() (map[string]string, error) {
	var bySlug map[string][]string
	if err := json.Unmarshal(aiMotionMapJSON, &bySlug); err != nil {
		return nil, fmt.Errorf("unmarshal ai motion map: %w", err)
	}

	byExercise := make(map[string]string)
	for motionSlug, exerciseIDs := range bySlug {
		for _, id := range exerciseIDs {
			byExercise[id] = motionSlug
		}
	}

	return byExercise, nil
}

func existingExerciseSlugs(ctx context.Context, db *sql.DB) (map[string]bool, error) {
	slugs := make(map[string]bool)
	if db == nil {
		return slugs, nil
	}

	rows, err := db.QueryContext(ctx, `SELECT slug FROM exercise.exercises`)
	if err != nil {
		return nil, fmt.Errorf("query existing exercise slugs: %w", err)
	}
	defer rows.Close()

	for rows.Next() {
		var slug string
		if err := rows.Scan(&slug); err != nil {
			return nil, fmt.Errorf("scan exercise slug: %w", err)
		}
		slugs[slug] = true
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate exercise slugs: %w", err)
	}

	return slugs, nil
}

// newCreateExerciseRequest normalizes one source record (docs/05 §4, §5.1):
// null equipment -> BODY_ONLY; extra primary muscles move to secondary;
// stretching or static -> DURATION; rest 15s stretching, 90s compound, else
// 60s; no images -> DRAFT. Duplicate secondary muscles are dropped by the
// Exercise aggregate.
func newCreateExerciseRequest(rec *upstreamExercise, motionSlug string) (*exercisemsg.CreateExerciseRequest, error) {
	if len(rec.PrimaryMuscles) == 0 {
		return nil, fmt.Errorf("exercise %s has no primary muscle", rec.ID)
	}

	primary, err := upstreamEnum(exercisemsg.MuscleGroup_value, "MUSCLE_GROUP_", rec.PrimaryMuscles[0])
	if err != nil {
		return nil, fmt.Errorf("exercise %s: %w", rec.ID, err)
	}
	extraMuscles := append(append([]string(nil), rec.PrimaryMuscles[1:]...), rec.SecondaryMuscles...)
	secondary := make([]exercisemsg.MuscleGroup, 0, len(extraMuscles))
	for _, m := range extraMuscles {
		v, muscleErr := upstreamEnum(exercisemsg.MuscleGroup_value, "MUSCLE_GROUP_", m)
		if muscleErr != nil {
			return nil, fmt.Errorf("exercise %s: %w", rec.ID, muscleErr)
		}
		secondary = append(secondary, exercisemsg.MuscleGroup(v))
	}

	equipment := "body only"
	if rec.Equipment != nil {
		equipment = *rec.Equipment
	}
	equipmentValue, err := upstreamEnum(exercisemsg.Equipment_value, "EQUIPMENT_", equipment)
	if err != nil {
		return nil, fmt.Errorf("exercise %s: %w", rec.ID, err)
	}
	category, err := upstreamEnum(exercisemsg.ExerciseCategory_value, "EXERCISE_CATEGORY_", rec.Category)
	if err != nil {
		return nil, fmt.Errorf("exercise %s: %w", rec.ID, err)
	}
	level, err := upstreamEnum(exercisemsg.ExerciseLevel_value, "EXERCISE_LEVEL_", rec.Level)
	if err != nil {
		return nil, fmt.Errorf("exercise %s: %w", rec.ID, err)
	}
	force, err := optionalUpstreamEnum(exercisemsg.ExerciseForce_value, "EXERCISE_FORCE_", rec.Force)
	if err != nil {
		return nil, fmt.Errorf("exercise %s: %w", rec.ID, err)
	}
	mechanic, err := optionalUpstreamEnum(exercisemsg.ExerciseMechanic_value, "EXERCISE_MECHANIC_", rec.Mechanic)
	if err != nil {
		return nil, fmt.Errorf("exercise %s: %w", rec.ID, err)
	}

	isStretching := exercisemsg.ExerciseCategory(category) == exercisemsg.ExerciseCategory_EXERCISE_CATEGORY_STRETCHING
	measurementType := exercisemsg.ExerciseMeasurementType_EXERCISE_MEASUREMENT_TYPE_REPS
	if isStretching || exercisemsg.ExerciseForce(force) == exercisemsg.ExerciseForce_EXERCISE_FORCE_STATIC {
		measurementType = exercisemsg.ExerciseMeasurementType_EXERCISE_MEASUREMENT_TYPE_DURATION
	}

	restSeconds := int32(defaultRestSeconds)
	switch {
	case isStretching:
		restSeconds = stretchingRestSeconds
	case exercisemsg.ExerciseMechanic(mechanic) == exercisemsg.ExerciseMechanic_EXERCISE_MECHANIC_COMPOUND:
		restSeconds = compoundRestSeconds
	}

	status := exercisemsg.ExerciseStatus_EXERCISE_STATUS_ACTIVE
	if len(rec.Images) == 0 {
		status = exercisemsg.ExerciseStatus_EXERCISE_STATUS_DRAFT
	}

	return &exercisemsg.CreateExerciseRequest{
		Slug:               rec.ID,
		Name:               strings.TrimSpace(rec.Name),
		PrimaryMuscle:      exercisemsg.MuscleGroup(primary),
		SecondaryMuscles:   secondary,
		Equipment:          exercisemsg.Equipment(equipmentValue),
		Category:           exercisemsg.ExerciseCategory(category),
		Level:              exercisemsg.ExerciseLevel(level),
		Force:              exercisemsg.ExerciseForce(force),
		Mechanic:           exercisemsg.ExerciseMechanic(mechanic),
		Instructions:       rec.Instructions,
		Images:             rec.Images,
		MeasurementType:    measurementType,
		DefaultRestSeconds: restSeconds,
		MotionSlug:         motionSlug,
		Status:             status,
	}, nil
}

// upstreamEnum resolves a source value ("lower back", "e-z curl bar") to the
// contract enum number named prefix + code ("LOWER_BACK", "EZ_CURL_BAR").
func upstreamEnum(values map[string]int32, prefix, raw string) (int32, error) {
	code := strings.ToUpper(strings.ReplaceAll(strings.ReplaceAll(strings.TrimSpace(raw), "-", ""), " ", "_"))
	v, ok := values[prefix+code]
	if !ok || v == 0 {
		return 0, fmt.Errorf("unknown source value %q for %s", raw, strings.TrimSuffix(prefix, "_"))
	}

	return v, nil
}

// optionalUpstreamEnum maps a null source value to UNSPECIFIED.
func optionalUpstreamEnum(values map[string]int32, prefix string, raw *string) (int32, error) {
	if raw == nil {
		return 0, nil
	}

	return upstreamEnum(values, prefix, *raw)
}
