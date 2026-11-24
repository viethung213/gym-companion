package seeder_test

import (
	"context"
	"database/sql"
	"fmt"
	"testing"

	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/viethung213/gym-companion/internal/shared/seeder"
	"gorm.io/gorm"
)

func setupMotionSpecTestDB(t *testing.T, dbName string) *sql.DB {
	t.Helper()

	dsn := fmt.Sprintf("file:%s?mode=memory&cache=shared", dbName)
	gormDB, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{
		SkipDefaultTransaction: true,
	})
	require.NoError(t, err)

	db, err := gormDB.DB()
	require.NoError(t, err)
	db.SetMaxOpenConns(1)

	schemaSQL := `
		ATTACH DATABASE ':memory:' AS exercise;
		ATTACH DATABASE ':memory:' AS workout_execution;

		CREATE TABLE IF NOT EXISTS exercise.exercises (
			id TEXT PRIMARY KEY,
			name TEXT NOT NULL,
			motion_slug TEXT,
			has_ai_supported INTEGER NOT NULL DEFAULT 0
		);

		CREATE TABLE IF NOT EXISTS workout_execution.motion_specifications (
			exercise_id TEXT PRIMARY KEY,
			exercise_name TEXT DEFAULT '',
			onnx_detector_url TEXT,
			onnx_skeleton_url TEXT,
			local_rules_url TEXT,
			dialogue_engine_url TEXT,
			recommended_camera_angle TEXT,
			is_ready INTEGER NOT NULL DEFAULT 0,
			created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
			updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
		);
	`
	_, err = db.Exec(schemaSQL)
	require.NoError(t, err)

	return db
}

func TestSeedAIMotionSpecifications_NilDBs(t *testing.T) {
	ctx := context.Background()
	db := setupMotionSpecTestDB(t, "nil_dbs_test")
	defer db.Close()

	err := seeder.SeedAIMotionSpecifications(ctx, nil, db)
	assert.ErrorContains(t, err, "workoutDB cannot be nil")

	err = seeder.SeedAIMotionSpecifications(ctx, db, nil)
	assert.ErrorContains(t, err, "exerciseDB cannot be nil")
}

func TestSeedAIMotionSpecifications_NoLinkedExercises(t *testing.T) {
	ctx := context.Background()
	db := setupMotionSpecTestDB(t, "no_match_test")
	defer db.Close()

	// A name that used to match an alias no longer links anything: only motion_slug does (F7).
	_, err := db.Exec(`INSERT INTO exercise.exercises (id, name, motion_slug) VALUES ('ex-1', 'Squat', NULL)`)
	require.NoError(t, err)

	require.NoError(t, seeder.SeedAIMotionSpecifications(ctx, db, db))

	var count int
	require.NoError(t, db.QueryRow(`SELECT COUNT(*) FROM workout_execution.motion_specifications`).Scan(&count))
	assert.Equal(t, 0, count)
}

func TestSeedAIMotionSpecifications_SuccessAndIdempotent(t *testing.T) {
	ctx := context.Background()
	db := setupMotionSpecTestDB(t, "success_test")
	defer db.Close()

	// The 16 source exercises of data/ai_motion_map.json (12 motion slugs).
	exercises := []struct {
		id         string
		motionSlug string
	}{
		{"Barbell_Bench_Press_-_Medium_Grip", "bench_press"},
		{"Dumbbell_Bicep_Curl", "bicep_curl"},
		{"Barbell_Deadlift", "deadlift"},
		{"Side_Lateral_Raise", "lateral_raise"},
		{"Dumbbell_Lunges", "lunge"},
		{"Barbell_Lunge", "lunge"},
		{"Plank", "plank"},
		{"Pullups", "pull_up"},
		{"Pushups", "push_up"},
		{"Dumbbell_Shoulder_Press", "shoulder_press"},
		{"Sit-Up", "sit_up"},
		{"3_4_Sit-Up", "sit_up"},
		{"Barbell_Squat", "squat"},
		{"Bodyweight_Squat", "squat"},
		{"Dips_-_Triceps_Version", "triceps_dip"},
		{"Bench_Dips", "triceps_dip"},
	}
	for _, ex := range exercises {
		_, err := db.Exec(`INSERT INTO exercise.exercises (id, name, motion_slug) VALUES ($1, $1, $2)`, ex.id, ex.motionSlug)
		require.NoError(t, err)
	}
	_, err := db.Exec(`INSERT INTO exercise.exercises (id, name) VALUES ('Romanian_Deadlift', 'Romanian Deadlift')`)
	require.NoError(t, err)

	for run := 1; run <= 2; run++ {
		require.NoError(t, seeder.SeedAIMotionSpecifications(ctx, db, db), "run %d", run)

		var specCount, aiCount, slugCount int
		require.NoError(t, db.QueryRow(`SELECT COUNT(*) FROM workout_execution.motion_specifications`).Scan(&specCount))
		require.NoError(t, db.QueryRow(`SELECT COUNT(*) FROM exercise.exercises WHERE has_ai_supported = 1`).Scan(&aiCount))
		require.NoError(t, db.QueryRow(
			`SELECT COUNT(DISTINCT motion_slug) FROM exercise.exercises WHERE has_ai_supported = 1`).Scan(&slugCount))
		assert.Equal(t, len(exercises), specCount, "run %d", run)
		assert.Equal(t, len(exercises), aiCount, "run %d", run)
		assert.Equal(t, 12, slugCount, "run %d", run)
	}

	var camAngle, ruleURL string
	var isReady bool
	err = db.QueryRow(`SELECT recommended_camera_angle, local_rules_url, is_ready FROM workout_execution.motion_specifications
		WHERE exercise_id = 'Pullups'`).Scan(&camAngle, &ruleURL, &isReady)
	require.NoError(t, err)
	assert.Equal(t, "FRONT", camAngle)
	assert.Equal(t, "rules/pull_up_rule.json", ruleURL)
	assert.True(t, isReady)

	err = db.QueryRow(`SELECT local_rules_url FROM workout_execution.motion_specifications
		WHERE exercise_id = '3_4_Sit-Up'`).Scan(&ruleURL)
	require.NoError(t, err)
	assert.Equal(t, "rules/sit_up_rule.json", ruleURL)

	var romanianAI int
	require.NoError(t, db.QueryRow(`SELECT has_ai_supported FROM exercise.exercises WHERE id = 'Romanian_Deadlift'`).Scan(&romanianAI))
	assert.Equal(t, 0, romanianAI, "Romanian deadlift is not linked to the deadlift rule (Q2)")
}

func TestSeedAIMotionSpecifications_UnknownMotionSlugSkipped(t *testing.T) {
	ctx := context.Background()
	db := setupMotionSpecTestDB(t, "unknown_slug_test")
	defer db.Close()

	_, err := db.Exec(`INSERT INTO exercise.exercises (id, name, motion_slug) VALUES ('ex-x', 'Burpee', 'burpee')`)
	require.NoError(t, err)

	require.NoError(t, seeder.SeedAIMotionSpecifications(ctx, db, db))

	var specCount, aiCount int
	require.NoError(t, db.QueryRow(`SELECT COUNT(*) FROM workout_execution.motion_specifications`).Scan(&specCount))
	require.NoError(t, db.QueryRow(`SELECT COUNT(*) FROM exercise.exercises WHERE has_ai_supported = 1`).Scan(&aiCount))
	assert.Equal(t, 0, specCount)
	assert.Equal(t, 0, aiCount)
}

func TestSeedAIMotionSpecifications_ContextCancelled(t *testing.T) {
	db := setupMotionSpecTestDB(t, "cancelled_test")
	defer db.Close()

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	assert.Error(t, seeder.SeedAIMotionSpecifications(ctx, db, db))
}

func TestSeedAIMotionSpecifications_DualPoolSelfHealing(t *testing.T) {
	ctx := context.Background()
	db := setupMotionSpecTestDB(t, "self_healing_test")
	defer db.Close()

	// Ready specifications committed in workoutDB while the exerciseDB update was missed.
	// Only the exercise with a motion slug may be flagged (chk_exercises_ai_requires_motion_slug).
	_, err := db.Exec(`
		INSERT INTO exercise.exercises (id, name, motion_slug) VALUES
			('ex-healed', 'Barbell Squat', 'squat'),
			('ex-unlinked', 'Custom Squat', NULL)`)
	require.NoError(t, err)
	_, err = db.Exec(`
		INSERT INTO workout_execution.motion_specifications (
			exercise_id, exercise_name, onnx_detector_url, onnx_skeleton_url, local_rules_url, dialogue_engine_url, recommended_camera_angle, is_ready
		) VALUES
			('ex-healed', 'Barbell Squat', '/m', '/m', 'rules/squat_rule.json', 'dialogues/squat_dialogue.json', 'SIDE', 1),
			('ex-unlinked', 'Custom Squat', '/m', '/m', 'rules/squat_rule.json', 'dialogues/squat_dialogue.json', 'SIDE', 1)
	`)
	require.NoError(t, err)

	require.NoError(t, seeder.SeedAIMotionSpecifications(ctx, db, db))

	var healed, unlinked int
	require.NoError(t, db.QueryRow(`SELECT has_ai_supported FROM exercise.exercises WHERE id = 'ex-healed'`).Scan(&healed))
	require.NoError(t, db.QueryRow(`SELECT has_ai_supported FROM exercise.exercises WHERE id = 'ex-unlinked'`).Scan(&unlinked))
	assert.Equal(t, 1, healed, "expected has_ai_supported to be healed to 1")
	assert.Equal(t, 0, unlinked, "an exercise without motion slug must not be flagged")
}
