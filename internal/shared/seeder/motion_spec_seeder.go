package seeder

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"log/slog"
	"sort"
	"strings"
)

type motionSpecTarget struct {
	slug        string
	cameraAngle string
}

func normalizeExerciseName(name string) string {
	return strings.ToLower(strings.TrimSpace(name))
}

// getExerciseMotionMappings returns canonical mapping from exercise names/aliases
// to their corresponding motion specification asset slug and recommended camera angle.
func getExerciseMotionMappings() map[string]motionSpecTarget {
	return map[string]motionSpecTarget{
		// Bicep Curl
		"dumbbell biceps curl": {slug: "bicep_curl", cameraAngle: "SIDE"},
		"dumbbell bicep curl":  {slug: "bicep_curl", cameraAngle: "SIDE"},
		"biceps curl":          {slug: "bicep_curl", cameraAngle: "SIDE"},
		"bicep curl":           {slug: "bicep_curl", cameraAngle: "SIDE"},

		// Lateral Raise
		"dumbbell lateral raise": {slug: "lateral_raise", cameraAngle: "FRONT"},
		"lateral raise":          {slug: "lateral_raise", cameraAngle: "FRONT"},

		// Lunge
		"barbell lunge":  {slug: "lunge", cameraAngle: "SIDE"},
		"dumbbell lunge": {slug: "lunge", cameraAngle: "SIDE"},
		"lunge":          {slug: "lunge", cameraAngle: "SIDE"},

		// Plank
		"weighted front plank":   {slug: "plank", cameraAngle: "SIDE"},
		"front plank with twist": {slug: "plank", cameraAngle: "SIDE"},
		"front plank":            {slug: "plank", cameraAngle: "SIDE"},
		"plank":                  {slug: "plank", cameraAngle: "SIDE"},

		// Pull-up
		"pull-up": {slug: "pull_up", cameraAngle: "FRONT"},
		"pull up": {slug: "pull_up", cameraAngle: "FRONT"},
		"pullup":  {slug: "pull_up", cameraAngle: "FRONT"},

		// Push-up
		"push-up": {slug: "push_up", cameraAngle: "SIDE"},
		"push up": {slug: "push_up", cameraAngle: "SIDE"},
		"pushup":  {slug: "push_up", cameraAngle: "SIDE"},

		// Shoulder Press
		"dumbbell seated shoulder press": {slug: "shoulder_press", cameraAngle: "FRONT"},
		"cable shoulder press":           {slug: "shoulder_press", cameraAngle: "FRONT"},
		"shoulder press":                 {slug: "shoulder_press", cameraAngle: "FRONT"},

		// Sit-up
		"3/4 sit-up": {slug: "sit_up", cameraAngle: "SIDE"},
		"sit-up":     {slug: "sit_up", cameraAngle: "SIDE"},
		"sit up":     {slug: "sit_up", cameraAngle: "SIDE"},
		"situp":      {slug: "sit_up", cameraAngle: "SIDE"},

		// Squat
		"weighted squat":     {slug: "squat", cameraAngle: "SIDE"},
		"barbell full squat": {slug: "squat", cameraAngle: "SIDE"},
		"squat":              {slug: "squat", cameraAngle: "SIDE"},

		// Bench Press
		"barbell bench press": {slug: "bench_press", cameraAngle: "SIDE"},
		"bench press":         {slug: "bench_press", cameraAngle: "SIDE"},

		// Deadlift
		"barbell deadlift":  {slug: "deadlift", cameraAngle: "SIDE"},
		"deadlift":          {slug: "deadlift", cameraAngle: "SIDE"},
		"romanian deadlift": {slug: "deadlift", cameraAngle: "SIDE"},

		// Triceps Dip
		"triceps dip":  {slug: "triceps_dip", cameraAngle: "SIDE"},
		"tricep dip":   {slug: "triceps_dip", cameraAngle: "SIDE"},
		"triceps dips": {slug: "triceps_dip", cameraAngle: "SIDE"},
		"tricep dips":  {slug: "triceps_dip", cameraAngle: "SIDE"},
	}
}

type matchedExercise struct {
	id   string
	name string
}

// SeedAIMotionSpecifications synchronizes motion specifications for the 12 AI-supported exercises:
// 1. Queries exerciseDB for exercise records matching supported biomechanical exercise names (case-insensitive).
// 2. Upserts matching records into workout_execution.motion_specifications on workoutDB (idempotent).
// 3. Updates has_ai_supported = TRUE on exerciseDB for all successfully provisioned exercise IDs.
// 4. Ensures any previously existing ready motion specs in workoutDB also have has_ai_supported synced.
// This completely avoids cross-schema queries and race conditions across isolated DB connection pools.
func SeedAIMotionSpecifications(ctx context.Context, workoutDB, exerciseDB *sql.DB) error {
	if workoutDB == nil {
		return errors.New("workoutDB cannot be nil")
	}
	if exerciseDB == nil {
		return errors.New("exerciseDB cannot be nil")
	}

	exerciseMotionMappings := getExerciseMotionMappings()
	names := make([]string, 0, len(exerciseMotionMappings))
	for name := range exerciseMotionMappings {
		names = append(names, name)
	}
	sort.Strings(names)

	placeholders := make([]string, len(names))
	args := make([]any, len(names))
	for i, name := range names {
		placeholders[i] = fmt.Sprintf("$%d", i+1)
		args[i] = name
	}

	query := fmt.Sprintf(
		"SELECT id, name FROM exercise.exercises WHERE LOWER(TRIM(name)) IN (%s)",
		strings.Join(placeholders, ", "),
	)

	rows, err := exerciseDB.QueryContext(ctx, query, args...)
	if err != nil {
		return fmt.Errorf("query exercises for ai motion specs: %w", err)
	}
	defer rows.Close()

	var matched []matchedExercise
	for rows.Next() {
		var ex matchedExercise
		if scanErr := rows.Scan(&ex.id, &ex.name); scanErr != nil {
			return fmt.Errorf("scan exercise row: %w", scanErr)
		}
		matched = append(matched, ex)
	}
	if err := rows.Err(); err != nil {
		return fmt.Errorf("rows iteration exercises: %w", err)
	}

	if len(matched) == 0 {
		slog.InfoContext(ctx, "No matching exercises found in exercise.exercises to seed motion specifications, checking existing specs")
	} else {
		slog.InfoContext(ctx, "Seeding AI motion specifications into workout_execution",
			slog.Int("count", len(matched)),
		)

		tx, err := workoutDB.BeginTx(ctx, nil)
		if err != nil {
			return fmt.Errorf("begin workout_execution transaction: %w", err)
		}
		defer func() { _ = tx.Rollback() }()

		stmt, err := tx.PrepareContext(ctx, `
			INSERT INTO workout_execution.motion_specifications (
				exercise_id, exercise_name, onnx_detector_url, onnx_skeleton_url,
				local_rules_url, dialogue_engine_url, recommended_camera_angle, is_ready, updated_at
			) VALUES (
				$1, $2, $3, $4, $5, $6, $7, $8, CURRENT_TIMESTAMP
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
			return fmt.Errorf("prepare motion specification upsert: %w", err)
		}
		defer stmt.Close()

		const (
			defaultDetectorURL = "/models/yolo11n-pose.onnx"
			defaultSkeletonURL = "/models/yolo11n-pose.onnx"
		)

		matchedIDs := make([]any, 0, len(matched))
		for _, ex := range matched {
			mapping, ok := exerciseMotionMappings[normalizeExerciseName(ex.name)]
			if !ok {
				continue
			}
			localRulesURL := fmt.Sprintf("rules/%s_rule.json", mapping.slug)
			dialogueURL := fmt.Sprintf("dialogues/%s_dialogue.json", mapping.slug)

			if _, execErr := stmt.ExecContext(ctx,
				ex.id,
				ex.name,
				defaultDetectorURL,
				defaultSkeletonURL,
				localRulesURL,
				dialogueURL,
				mapping.cameraAngle,
				true,
			); execErr != nil {
				return fmt.Errorf("upsert motion specification for exercise %s (%s): %w", ex.name, ex.id, execErr)
			}
			matchedIDs = append(matchedIDs, ex.id)
		}

		if err := tx.Commit(); err != nil {
			return fmt.Errorf("commit workout_execution transaction: %w", err)
		}

		if len(matchedIDs) > 0 {
			updatePlaceholders := make([]string, len(matchedIDs))
			for i := range matchedIDs {
				updatePlaceholders[i] = fmt.Sprintf("$%d", i+1)
			}
			updateQuery := fmt.Sprintf(
				"UPDATE exercise.exercises SET has_ai_supported = TRUE WHERE id IN (%s)",
				strings.Join(updatePlaceholders, ", "),
			)
			if _, updateErr := exerciseDB.ExecContext(ctx, updateQuery, matchedIDs...); updateErr != nil {
				return fmt.Errorf("update exercise has_ai_supported: %w", updateErr)
			}
		}

		slog.InfoContext(ctx, "Successfully seeded AI motion specifications and synced has_ai_supported flags",
			slog.Int("count", len(matchedIDs)),
		)
	}

	// Dual-pool self-healing sync: Ensure any existing ready motion specifications in workoutDB
	// also have has_ai_supported set to TRUE in exerciseDB (repairs partial commits across network partitions).
	if existingRows, qErr := workoutDB.QueryContext(ctx, "SELECT exercise_id FROM workout_execution.motion_specifications WHERE is_ready = TRUE"); qErr != nil {
		slog.WarnContext(ctx, "Querying existing motion specifications failed", slog.String("error", qErr.Error()))
	} else {
		defer existingRows.Close()
		var readyIDs []any
		for existingRows.Next() {
			var id string
			if scanErr := existingRows.Scan(&id); scanErr == nil && id != "" {
				readyIDs = append(readyIDs, id)
			}
		}
		if err := existingRows.Err(); err != nil {
			slog.WarnContext(ctx, "Iterating existing motion specifications failed", slog.String("error", err.Error()))
		} else if len(readyIDs) > 0 {
			placeholders := make([]string, len(readyIDs))
			for i := range readyIDs {
				placeholders[i] = fmt.Sprintf("$%d", i+1)
			}
			syncQuery := fmt.Sprintf(
				"UPDATE exercise.exercises SET has_ai_supported = TRUE WHERE id IN (%s)",
				strings.Join(placeholders, ", "),
			)
			if _, syncErr := exerciseDB.ExecContext(ctx, syncQuery, readyIDs...); syncErr != nil {
				slog.WarnContext(ctx, "Syncing existing has_ai_supported failed", slog.String("error", syncErr.Error()))
			}
		}
	}

	return nil
}
