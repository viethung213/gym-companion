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

func TestSeedAIMotionSpecifications_NoMatchingExercises(t *testing.T) {
	ctx := context.Background()
	db := setupMotionSpecTestDB(t, "no_match_test")
	defer db.Close()

	// Insert irrelevant exercises
	_, err := db.Exec(`INSERT INTO exercise.exercises (id, name, has_ai_supported) VALUES ('ex-1', 'Unknown Exercise', 0)`)
	require.NoError(t, err)

	err = seeder.SeedAIMotionSpecifications(ctx, db, db)
	require.NoError(t, err)

	var count int
	err = db.QueryRow(`SELECT COUNT(*) FROM workout_execution.motion_specifications`).Scan(&count)
	require.NoError(t, err)
	assert.Equal(t, 0, count)
}

func TestSeedAIMotionSpecifications_SuccessAndIdempotent(t *testing.T) {
	ctx := context.Background()
	db := setupMotionSpecTestDB(t, "success_test")
	defer db.Close()

	// Insert all supported exercises
	exercises := []struct {
		id   string
		name string
	}{
		{"ex-bicep", "Dumbbell Biceps Curl"},
		{"ex-lat-raise", "Lateral Raise"},
		{"ex-lunge", "Barbell Lunge"},
		{"ex-plank", "Plank"},
		{"ex-pullup", "Pull-up"},
		{"ex-pushup", "Push-up"},
		{"ex-shoulder", "Shoulder Press"},
		{"ex-situp", "Sit-up"},
		{"ex-squat", "Squat"},
		{"ex-bench", "Barbell Bench Press"},
		{"ex-deadlift", "Barbell Deadlift"},
		{"ex-dip", "Triceps Dip"},
	}

	for _, ex := range exercises {
		_, err := db.Exec(`INSERT INTO exercise.exercises (id, name, has_ai_supported) VALUES ($1, $2, 0)`, ex.id, ex.name)
		require.NoError(t, err)
	}

	// 1. First run: should insert 12 motion specs and update has_ai_supported
	err := seeder.SeedAIMotionSpecifications(ctx, db, db)
	require.NoError(t, err)

	var specCount int
	err = db.QueryRow(`SELECT COUNT(*) FROM workout_execution.motion_specifications`).Scan(&specCount)
	require.NoError(t, err)
	assert.Equal(t, 12, specCount)

	// Verify specific exercises camera angles and rule paths
	var camAngle, ruleURL string
	var isReady bool
	err = db.QueryRow(`SELECT recommended_camera_angle, local_rules_url, is_ready FROM workout_execution.motion_specifications WHERE exercise_id = 'ex-pullup'`).Scan(&camAngle, &ruleURL, &isReady)
	require.NoError(t, err)
	assert.Equal(t, "FRONT", camAngle)
	assert.Equal(t, "rules/pull_up_rule.json", ruleURL)
	assert.True(t, isReady)

	err = db.QueryRow(`SELECT recommended_camera_angle, local_rules_url FROM workout_execution.motion_specifications WHERE exercise_id = 'ex-bicep'`).Scan(&camAngle, &ruleURL)
	require.NoError(t, err)
	assert.Equal(t, "SIDE", camAngle)
	assert.Equal(t, "rules/bicep_curl_rule.json", ruleURL)

	// Verify has_ai_supported was updated in exercise.exercises
	var aiSupportedCount int
	err = db.QueryRow(`SELECT COUNT(*) FROM exercise.exercises WHERE has_ai_supported = 1`).Scan(&aiSupportedCount)
	require.NoError(t, err)
	assert.Equal(t, 12, aiSupportedCount)

	// 2. Second run: idempotency test, should succeed without error
	err = seeder.SeedAIMotionSpecifications(ctx, db, db)
	require.NoError(t, err)

	err = db.QueryRow(`SELECT COUNT(*) FROM workout_execution.motion_specifications`).Scan(&specCount)
	require.NoError(t, err)
	assert.Equal(t, 12, specCount)
}

func TestSeedAIMotionSpecifications_ContextCancelled(t *testing.T) {
	db := setupMotionSpecTestDB(t, "cancelled_test")
	defer db.Close()

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	err := seeder.SeedAIMotionSpecifications(ctx, db, db)
	assert.Error(t, err)
}

func TestSeedAIMotionSpecifications_CaseAndWhitespaceInsensitive(t *testing.T) {
	ctx := context.Background()
	db := setupMotionSpecTestDB(t, "case_insensitive_test")
	defer db.Close()

	// Insert exercises with mixed casing, aliases, and whitespace
	exercises := []struct {
		id   string
		name string
	}{
		{"ex-lower-pullup", "pull-up"},
		{"ex-upper-pushup", "PUSH-UP"},
		{"ex-spaced-bicep", "  Dumbbell Biceps Curl  "},
		{"ex-bench-alias", "Bench Press"},
		{"ex-deadlift-alias", "Deadlift"},
	}

	for _, ex := range exercises {
		_, err := db.Exec(`INSERT INTO exercise.exercises (id, name, has_ai_supported) VALUES ($1, $2, 0)`, ex.id, ex.name)
		require.NoError(t, err)
	}

	err := seeder.SeedAIMotionSpecifications(ctx, db, db)
	require.NoError(t, err)

	var specCount int
	err = db.QueryRow(`SELECT COUNT(*) FROM workout_execution.motion_specifications`).Scan(&specCount)
	require.NoError(t, err)
	assert.Equal(t, len(exercises), specCount)

	// Verify has_ai_supported was updated for all
	var aiSupportedCount int
	err = db.QueryRow(`SELECT COUNT(*) FROM exercise.exercises WHERE has_ai_supported = 1`).Scan(&aiSupportedCount)
	require.NoError(t, err)
	assert.Equal(t, len(exercises), aiSupportedCount)
}

func TestSeedAIMotionSpecifications_DualPoolSelfHealing(t *testing.T) {
	ctx := context.Background()
	db := setupMotionSpecTestDB(t, "self_healing_test")
	defer db.Close()

	// Scenario: Motion specification was committed in workoutDB, but exerciseDB update was missed
	// (e.g. partial failure or network glitch)
	_, err := db.Exec(`INSERT INTO exercise.exercises (id, name, has_ai_supported) VALUES ('ex-healed', 'Custom Squat', 0)`)
	require.NoError(t, err)

	_, err = db.Exec(`
		INSERT INTO workout_execution.motion_specifications (
			exercise_id, exercise_name, onnx_detector_url, onnx_skeleton_url, local_rules_url, dialogue_engine_url, recommended_camera_angle, is_ready
		) VALUES (
			'ex-healed', 'Custom Squat', '/models/yolo.onnx', '/models/yolo.onnx', 'rules/squat_rule.json', 'dialogues/squat_dialogue.json', 'SIDE', 1
		)
	`)
	require.NoError(t, err)

	// Run seeder: it should detect the existing ready motion specification and sync has_ai_supported = 1
	err = seeder.SeedAIMotionSpecifications(ctx, db, db)
	require.NoError(t, err)

	var hasAI int
	err = db.QueryRow(`SELECT has_ai_supported FROM exercise.exercises WHERE id = 'ex-healed'`).Scan(&hasAI)
	require.NoError(t, err)
	assert.Equal(t, 1, hasAI, "expected has_ai_supported to be healed to 1")
}
