package adapters

import (
	"context"
	"testing"
	"time"

	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/viethung213/gym-companion/internal/coaching/application/port"
	"gorm.io/gorm"
)

func setupTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open("file::memory:?cache=shared"), &gorm.Config{})
	require.NoError(t, err)

	// Attach schemas for SQLite to support multi-schema queries in test
	err = db.Exec(`
		ATTACH DATABASE ':memory:' AS profile;
		ATTACH DATABASE ':memory:' AS workout_execution;
		ATTACH DATABASE ':memory:' AS exercise;
	`).Error
	require.NoError(t, err)

	// Create test tables in attached schemas
	err = db.Exec(`
		CREATE TABLE IF NOT EXISTS profile.users (
			user_id TEXT PRIMARY KEY,
			date_of_birth TIMESTAMP,
			experience_level TEXT,
			goals TEXT,
			preferred_workout_times TEXT,
			available_equipment TEXT,
			preferred_muscle_groups TEXT,
			updated_at TIMESTAMP
		);
		CREATE TABLE IF NOT EXISTS profile.body_metrics (
			id TEXT PRIMARY KEY,
			user_id TEXT,
			weight_kg REAL,
			height_cm REAL,
			logged_at TIMESTAMP
		);
		CREATE TABLE IF NOT EXISTS profile.injuries (
			id TEXT PRIMARY KEY,
			user_id TEXT,
			muscle_group TEXT,
			notes TEXT,
			reported_at TIMESTAMP,
			is_recovered BOOLEAN,
			recovered_at TIMESTAMP
		);
		CREATE TABLE IF NOT EXISTS workout_execution.workout_sessions (
			id TEXT PRIMARY KEY,
			user_id TEXT,
			plan_id TEXT,
			status TEXT,
			total_sets INTEGER,
			total_volume REAL,
			average_form_score REAL,
			average_rpe REAL,
			scheduled_at TIMESTAMP,
			started_at TIMESTAMP,
			ended_at TIMESTAMP
		);
		CREATE TABLE IF NOT EXISTS workout_execution.workout_set_logs (
			id TEXT PRIMARY KEY,
			session_id TEXT,
			set_number INTEGER,
			exercise_id TEXT,
			target_reps INTEGER,
			actual_reps INTEGER,
			weight REAL,
			form_score REAL,
			rpe REAL,
			created_at TIMESTAMP
		);
		CREATE TABLE IF NOT EXISTS workout_execution.personal_records (
			id TEXT PRIMARY KEY,
			user_id TEXT,
			exercise_id TEXT,
			one_rep_max REAL,
			weight REAL,
			reps INTEGER,
			achieved_at TIMESTAMP
		);
		CREATE TABLE IF NOT EXISTS exercise.exercises (
			id TEXT PRIMARY KEY,
			name TEXT,
			body_part_id TEXT,
			equipment_id TEXT,
			target_muscle_id TEXT,
			difficulty TEXT,
			status TEXT
		);
	`).Error
	require.NoError(t, err)

	return db
}

func TestPostgresUserProfileReader(t *testing.T) {
	ctx := context.Background()

	t.Run("returns fallback for non-existent user profile", func(t *testing.T) {
		db := setupTestDB(t)
		reader := NewPostgresUserProfileReader(db)

		profile, err := reader.GetProfile(ctx, "non-existent-user")
		require.NoError(t, err)
		assert.Equal(t, "non-existent-user", profile.UserID)
		assert.Equal(t, 75.0, profile.WeightKg)
		assert.Equal(t, "hypertrophy", profile.PrimaryGoal)
	})

	t.Run("reads real user profile and body metrics from DB", func(t *testing.T) {
		db := setupTestDB(t)
		reader := NewPostgresUserProfileReader(db)

		dob := time.Date(1995, 5, 20, 0, 0, 0, 0, time.UTC)
		err := db.Exec(`
			INSERT INTO profile.users (user_id, date_of_birth, experience_level, goals, available_equipment, preferred_muscle_groups, preferred_workout_times, updated_at)
			VALUES ('user-50kg', ?, 'INTERMEDIATE', '["strength"]', '["barbell","dumbbell"]', '["chest","back"]', '[{"day_of_week":1,"start_time":"07:00","end_time":"08:30"}]', CURRENT_TIMESTAMP);

			INSERT INTO profile.body_metrics (id, user_id, weight_kg, height_cm, logged_at)
			VALUES ('bm-1', 'user-50kg', 52.5, 165.0, CURRENT_TIMESTAMP);

			INSERT INTO profile.injuries (id, user_id, muscle_group, notes, reported_at, is_recovered)
			VALUES ('inj-1', 'user-50kg', 'shoulder', 'Mild strain', CURRENT_TIMESTAMP, 0);
		`, dob).Error
		require.NoError(t, err)

		profile, err := reader.GetProfile(ctx, "user-50kg")
		require.NoError(t, err)

		assert.Equal(t, "user-50kg", profile.UserID)
		assert.Equal(t, 52.5, profile.WeightKg)
		assert.Equal(t, 165.0, profile.HeightCm)
		assert.Equal(t, "strength", profile.PrimaryGoal)
		assert.Equal(t, []string{"barbell", "dumbbell"}, profile.AvailableEquipment)
		assert.Equal(t, []string{"chest", "back"}, profile.PreferredMuscleGroups)
		assert.Len(t, profile.AvailableSlots, 1)
		assert.Equal(t, time.Monday, profile.AvailableSlots[0].DayOfWeek)
		assert.Len(t, profile.ActiveInjuries, 1)
		assert.Equal(t, "shoulder", profile.ActiveInjuries[0].MuscleGroup)
	})
}

func TestPostgresWorkoutSessionReader(t *testing.T) {
	ctx := context.Background()

	t.Run("returns empty slice for user with no set logs or PRs", func(t *testing.T) {
		db := setupTestDB(t)
		reader := NewPostgresWorkoutSessionReader(db)

		logs, err := reader.GetSetLogs(ctx, "new-user", "bench-press", 5)
		require.NoError(t, err)
		assert.Empty(t, logs, "new user with no logs should return empty slice, not hardcoded 80kg")
	})

	t.Run("reads set logs and personal records from DB", func(t *testing.T) {
		db := setupTestDB(t)
		reader := NewPostgresWorkoutSessionReader(db)

		now := time.Now()
		err := db.Exec(`
			INSERT INTO workout_execution.workout_sessions (id, user_id, plan_id, status, total_sets, average_rpe, average_form_score, ended_at)
			VALUES ('sess-1', 'user-1', 'plan-1', 'COMPLETED', 4, 8.5, 90.0, ?);

			INSERT INTO workout_execution.workout_set_logs (id, session_id, set_number, exercise_id, target_reps, actual_reps, weight, rpe, created_at)
			VALUES ('set-1', 'sess-1', 1, 'squat', 5, 5, 120.0, 8.5, ?);
		`, now, now).Error
		require.NoError(t, err)

		sessions, err := reader.GetRecentSessions(ctx, "user-1", time.Time{})
		require.NoError(t, err)
		assert.Len(t, sessions, 1)
		assert.Equal(t, "sess-1", sessions[0].SessionID)
		assert.Equal(t, 4, sessions[0].TotalSets)

		setLogs, err := reader.GetSetLogs(ctx, "user-1", "squat", 10)
		require.NoError(t, err)
		assert.Len(t, setLogs, 1)
		assert.Equal(t, "squat", setLogs[0].ExerciseID)
		assert.Equal(t, 120.0, setLogs[0].Weight)
		assert.Equal(t, 5, setLogs[0].Reps)
	})

	t.Run("falls back to personal records table when no set logs exist", func(t *testing.T) {
		db := setupTestDB(t)
		reader := NewPostgresWorkoutSessionReader(db)

		now := time.Now()
		err := db.Exec(`
			INSERT INTO workout_execution.personal_records (id, user_id, exercise_id, one_rep_max, weight, reps, achieved_at)
			VALUES ('pr-1', 'user-pr', 'deadlift', 160.0, 140.0, 3, ?);
		`, now).Error
		require.NoError(t, err)

		setLogs, err := reader.GetSetLogs(ctx, "user-pr", "deadlift", 5)
		require.NoError(t, err)
		assert.Len(t, setLogs, 1)
		assert.Equal(t, "deadlift", setLogs[0].ExerciseID)
		assert.Equal(t, 140.0, setLogs[0].Weight)
		assert.Equal(t, 3, setLogs[0].Reps)
	})
}

func TestPostgresExerciseCatalogReader(t *testing.T) {
	ctx := context.Background()

	t.Run("searches and filters exercises in catalog DB", func(t *testing.T) {
		db := setupTestDB(t)
		reader := NewPostgresExerciseCatalogReader(db)

		err := db.Exec(`
			INSERT INTO exercise.exercises (id, name, body_part_id, equipment_id, target_muscle_id, difficulty, status)
			VALUES
				('barbell-incline-bench-press', 'Barbell Incline Bench Press', 'chest', 'barbell', 'chest', 'Intermediate', 'ACTIVE'),
				('lat-pulldown-machine', 'Lat Pulldown', 'back', 'cable', 'lats', 'Beginner', 'ACTIVE');
		`).Error
		require.NoError(t, err)

		// Filter by chest
		chestExs, err := reader.SearchByFilter(ctx, &port.ExerciseFilter{TargetMuscleID: "chest"})
		require.NoError(t, err)
		assert.Len(t, chestExs, 1)
		assert.Equal(t, "barbell-incline-bench-press", chestExs[0].ExerciseID)
		assert.False(t, chestExs[0].IsMachineOrCable)

		// Get by ID
		ex, err := reader.GetByID(ctx, "barbell-incline-bench-press")
		require.NoError(t, err)
		assert.Equal(t, "Barbell Incline Bench Press", ex.Name)
	})

	t.Run("returns mock catalog fallback when database table is unseeded", func(t *testing.T) {
		db := setupTestDB(t)
		reader := NewPostgresExerciseCatalogReader(db)

		// Database has empty exercise table
		exs, err := reader.SearchByFilter(ctx, &port.ExerciseFilter{TargetMuscleID: "chest"})
		require.NoError(t, err)
		assert.NotEmpty(t, exs, "unseeded DB falls back to default catalog")
	})
}
