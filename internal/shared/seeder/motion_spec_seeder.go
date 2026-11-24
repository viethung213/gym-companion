package seeder

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"log/slog"
	"strings"
)

const (
	defaultDetectorURL = "/models/yolo11n-pose.onnx"
	defaultSkeletonURL = "/models/yolo11n-pose.onnx"
)

// motionCameraAngles is the recommended camera angle per AI motion_slug; every
// slug in data/ai_motion_map.json must be listed.
func motionCameraAngles() map[string]string {
	return map[string]string{
		"bench_press":    "SIDE",
		"bicep_curl":     "SIDE",
		"deadlift":       "SIDE",
		"lateral_raise":  "FRONT",
		"lunge":          "SIDE",
		"plank":          "SIDE",
		"pull_up":        "FRONT",
		"push_up":        "SIDE",
		"shoulder_press": "FRONT",
		"sit_up":         "SIDE",
		"squat":          "SIDE",
		"triceps_dip":    "SIDE",
	}
}

type motionExercise struct {
	id         string
	name       string
	motionSlug string
}

// SeedAIMotionSpecifications provisions a ready motion specification for every
// exercise linked to an AI motion via exercises.motion_slug (docs/05 §8.8):
// 1. Reads the linked exercises from exerciseDB.
// 2. Upserts their specifications into workout_execution.motion_specifications (idempotent).
// 3. Sets has_ai_supported = TRUE for the provisioned exercises.
// 4. Re-syncs has_ai_supported for ready specs left by an earlier partial run.
// The two databases are written separately, so no cross-schema query is needed.
func SeedAIMotionSpecifications(ctx context.Context, workoutDB, exerciseDB *sql.DB) error {
	if workoutDB == nil {
		return errors.New("workoutDB cannot be nil")
	}
	if exerciseDB == nil {
		return errors.New("exerciseDB cannot be nil")
	}

	linked, err := queryMotionExercises(ctx, exerciseDB)
	if err != nil {
		return err
	}

	provisioned, err := upsertMotionSpecifications(ctx, workoutDB, linked)
	if err != nil {
		return err
	}
	if err = markAISupported(ctx, exerciseDB, provisioned); err != nil {
		return fmt.Errorf("update exercise has_ai_supported: %w", err)
	}
	slog.InfoContext(ctx, "Seeded AI motion specifications",
		slog.Int("linked_exercises", len(linked)),
		slog.Int("provisioned", len(provisioned)),
	)

	readyIDs, err := queryReadySpecExerciseIDs(ctx, workoutDB)
	if err != nil {
		return err
	}
	if err := markAISupported(ctx, exerciseDB, readyIDs); err != nil {
		return fmt.Errorf("sync has_ai_supported for ready specifications: %w", err)
	}

	return nil
}

func queryMotionExercises(ctx context.Context, exerciseDB *sql.DB) ([]motionExercise, error) {
	rows, err := exerciseDB.QueryContext(ctx,
		`SELECT id, name, motion_slug FROM exercise.exercises WHERE motion_slug IS NOT NULL ORDER BY id`)
	if err != nil {
		return nil, fmt.Errorf("query exercises linked to ai motions: %w", err)
	}
	defer rows.Close()

	var linked []motionExercise
	for rows.Next() {
		var ex motionExercise
		if err := rows.Scan(&ex.id, &ex.name, &ex.motionSlug); err != nil {
			return nil, fmt.Errorf("scan exercise row: %w", err)
		}
		linked = append(linked, ex)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate exercises linked to ai motions: %w", err)
	}

	return linked, nil
}

// upsertMotionSpecifications returns the ids of the exercises it provisioned.
// A motion_slug without rule assets is skipped and logged.
func upsertMotionSpecifications(ctx context.Context, workoutDB *sql.DB, linked []motionExercise) ([]string, error) {
	if len(linked) == 0 {
		return nil, nil
	}

	tx, err := workoutDB.BeginTx(ctx, nil)
	if err != nil {
		return nil, fmt.Errorf("begin workout_execution transaction: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	stmt, err := tx.PrepareContext(ctx, `
		INSERT INTO workout_execution.motion_specifications (
			exercise_id, exercise_name, onnx_detector_url, onnx_skeleton_url,
			local_rules_url, dialogue_engine_url, recommended_camera_angle, is_ready, updated_at
		) VALUES (
			$1, $2, $3, $4, $5, $6, $7, TRUE, CURRENT_TIMESTAMP
		) ON CONFLICT (exercise_id) DO UPDATE SET
			exercise_name = EXCLUDED.exercise_name,
			onnx_detector_url = EXCLUDED.onnx_detector_url,
			onnx_skeleton_url = EXCLUDED.onnx_skeleton_url,
			local_rules_url = EXCLUDED.local_rules_url,
			dialogue_engine_url = EXCLUDED.dialogue_engine_url,
			recommended_camera_angle = EXCLUDED.recommended_camera_angle,
			is_ready = TRUE,
			updated_at = CURRENT_TIMESTAMP
	`)
	if err != nil {
		return nil, fmt.Errorf("prepare motion specification upsert: %w", err)
	}
	defer stmt.Close()

	angles := motionCameraAngles()
	provisioned := make([]string, 0, len(linked))
	for _, ex := range linked {
		angle, ok := angles[ex.motionSlug]
		if !ok {
			slog.WarnContext(ctx, "Skipping exercise linked to an unknown AI motion",
				slog.String("exercise_id", ex.id), slog.String("motion_slug", ex.motionSlug))
			continue
		}

		if _, err := stmt.ExecContext(ctx,
			ex.id,
			ex.name,
			defaultDetectorURL,
			defaultSkeletonURL,
			"rules/"+ex.motionSlug+"_rule.json",
			"dialogues/"+ex.motionSlug+"_dialogue.json",
			angle,
		); err != nil {
			return nil, fmt.Errorf("upsert motion specification for exercise %s: %w", ex.id, err)
		}
		provisioned = append(provisioned, ex.id)
	}

	if err := tx.Commit(); err != nil {
		return nil, fmt.Errorf("commit workout_execution transaction: %w", err)
	}

	return provisioned, nil
}

func queryReadySpecExerciseIDs(ctx context.Context, workoutDB *sql.DB) ([]string, error) {
	rows, err := workoutDB.QueryContext(ctx,
		`SELECT exercise_id FROM workout_execution.motion_specifications WHERE is_ready = TRUE`)
	if err != nil {
		return nil, fmt.Errorf("query ready motion specifications: %w", err)
	}
	defer rows.Close()

	var ids []string
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return nil, fmt.Errorf("scan motion specification row: %w", err)
		}
		ids = append(ids, id)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate ready motion specifications: %w", err)
	}

	return ids, nil
}

// markAISupported flags the given exercises, limited to those with a motion
// slug as the schema requires (chk_exercises_ai_requires_motion_slug).
func markAISupported(ctx context.Context, exerciseDB *sql.DB, ids []string) error {
	if len(ids) == 0 {
		return nil
	}

	placeholders := make([]string, len(ids))
	args := make([]any, len(ids))
	for i, id := range ids {
		placeholders[i] = fmt.Sprintf("$%d", i+1)
		args[i] = id
	}
	query := "UPDATE exercise.exercises SET has_ai_supported = TRUE WHERE motion_slug IS NOT NULL AND id IN (" +
		strings.Join(placeholders, ", ") + ")"
	if _, err := exerciseDB.ExecContext(ctx, query, args...); err != nil {
		return err
	}

	return nil
}
