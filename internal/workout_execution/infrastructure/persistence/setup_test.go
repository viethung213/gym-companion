//go:build integration

package persistence_test

import (
	"testing"

	"github.com/viethung213/gym-companion/internal/shared/database"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

func ensureTablesExist(db *gorm.DB) {
	db.Exec(`CREATE SCHEMA IF NOT EXISTS workout_execution;`)

	db.Exec(`CREATE TABLE IF NOT EXISTS workout_execution.workout_sessions (
		id VARCHAR(255) PRIMARY KEY,
		user_id VARCHAR(255) NOT NULL,
		plan_id VARCHAR(255) NOT NULL,
		status VARCHAR(50) NOT NULL DEFAULT 'SCHEDULED',
		total_sets INT DEFAULT 0,
		total_volume NUMERIC(10, 2) DEFAULT 0.0,
		average_form_score NUMERIC(5, 2),
		average_rpe NUMERIC(5, 2),
		scheduled_at TIMESTAMP WITH TIME ZONE,
		started_at TIMESTAMP WITH TIME ZONE,
		ended_at TIMESTAMP WITH TIME ZONE,
		created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
		updated_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
		version INT NOT NULL DEFAULT 1,
		is_shared BOOLEAN NOT NULL DEFAULT FALSE
	);`)
	db.Exec(`ALTER TABLE workout_execution.workout_sessions ADD COLUMN IF NOT EXISTS version INT NOT NULL DEFAULT 1;`)
	db.Exec(`ALTER TABLE workout_execution.workout_sessions ADD COLUMN IF NOT EXISTS is_shared BOOLEAN NOT NULL DEFAULT FALSE;`)
	db.Exec(`CREATE UNIQUE INDEX IF NOT EXISTS uq_workout_sessions_active_user ON workout_execution.workout_sessions(user_id) WHERE status = 'IN_PROGRESS';`)

	db.Exec(`CREATE TABLE IF NOT EXISTS workout_execution.workout_set_logs (
		id VARCHAR(255) PRIMARY KEY,
		session_id VARCHAR(255) NOT NULL REFERENCES workout_execution.workout_sessions(id) ON DELETE CASCADE,
		set_number INT NOT NULL,
		exercise_id VARCHAR(255) NOT NULL,
		target_reps INT NOT NULL DEFAULT 0,
		actual_reps INT NOT NULL DEFAULT 0,
		weight NUMERIC(10, 2) NOT NULL DEFAULT 0.0,
		form_score NUMERIC(5, 2),
		rpe NUMERIC(5, 2) DEFAULT 0.0,
		camera_angle VARCHAR(50),
		created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP
	);`)

	db.Exec(`CREATE TABLE IF NOT EXISTS workout_execution.rep_logs (
		id VARCHAR(255) PRIMARY KEY,
		set_log_id VARCHAR(255) NOT NULL REFERENCES workout_execution.workout_set_logs(id) ON DELETE CASCADE,
		rep_number INT NOT NULL,
		rom_percentage NUMERIC(5, 2) NOT NULL,
		error_codes JSONB,
		joint_angles JSONB,
		created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP
	);`)

	db.Exec(`CREATE TABLE IF NOT EXISTS workout_execution.session_errors (
		id VARCHAR(255) PRIMARY KEY,
		session_id VARCHAR(255) NOT NULL REFERENCES workout_execution.workout_sessions(id) ON DELETE CASCADE,
		set_number INT NOT NULL,
		rep_number INT NOT NULL,
		exercise_id VARCHAR(255) NOT NULL,
		error_code VARCHAR(100) NOT NULL,
		severity VARCHAR(50) NOT NULL,
		timestamp TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP
	);`)

	db.Exec(`CREATE TABLE IF NOT EXISTS workout_execution.personal_records (
		id VARCHAR(255) PRIMARY KEY,
		user_id VARCHAR(255) NOT NULL,
		exercise_id VARCHAR(255) NOT NULL,
		one_rep_max NUMERIC(10, 2) NOT NULL,
		weight NUMERIC(10, 2) NOT NULL,
		reps INT NOT NULL,
		form_verified BOOLEAN DEFAULT TRUE,
		achieved_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
		created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
		updated_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP
	);`)
	db.Exec(`CREATE UNIQUE INDEX IF NOT EXISTS uq_user_exercise_pr ON workout_execution.personal_records(user_id, exercise_id);`)

	db.Exec("CREATE EXTENSION IF NOT EXISTS pg_trgm;")
	db.Exec(`CREATE TABLE IF NOT EXISTS workout_execution.motion_specifications (
		exercise_id VARCHAR(255) PRIMARY KEY,
		exercise_name VARCHAR(255) DEFAULT '',
		onnx_detector_url VARCHAR(1024),
		onnx_skeleton_url VARCHAR(1024),
		local_rules_url VARCHAR(1024),
		dialogue_engine_url VARCHAR(1024),
		recommended_camera_angle VARCHAR(50),
		is_ready BOOLEAN NOT NULL DEFAULT FALSE,
		created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
		updated_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP
	);`)
	db.Exec(`CREATE INDEX IF NOT EXISTS idx_motion_specifications_name_trgm ON workout_execution.motion_specifications USING gin (exercise_name gin_trgm_ops);`)

	db.Exec(`CREATE TABLE IF NOT EXISTS workout_execution.outbox (
		id VARCHAR(255) PRIMARY KEY,
		event_id VARCHAR(255),
		aggregate_type VARCHAR(255),
		aggregate_id VARCHAR(255),
		event_type VARCHAR(255) NOT NULL,
		payload JSONB NOT NULL,
		partition_key VARCHAR(255),
		published BOOLEAN DEFAULT FALSE,
		published_at TIMESTAMP WITH TIME ZONE,
		created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP
	);`)
	db.Exec("ALTER TABLE workout_execution.outbox ADD COLUMN IF NOT EXISTS status VARCHAR(50) DEFAULT 'PENDING' NOT NULL;")
	db.Exec("ALTER TABLE workout_execution.outbox ADD COLUMN IF NOT EXISTS locked_until TIMESTAMP WITH TIME ZONE;")
	db.Exec(`CREATE INDEX IF NOT EXISTS idx_workout_execution_outbox_pub_created ON workout_execution.outbox (published, created_at);`)

	db.Exec(`CREATE TABLE IF NOT EXISTS workout_execution.outbox_log (
		id UUID PRIMARY KEY,
		event_id UUID NOT NULL,
		event_type VARCHAR(255) NOT NULL,
		payload JSONB NOT NULL,
		partition_key VARCHAR(255) NOT NULL,
		processed_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
		status VARCHAR(50) NOT NULL,
		error_message TEXT
	);`)
}

func getTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	sqlDB, err := database.GetRegistry().GetPool("workout_execution")
	if err != nil {
		t.Fatalf("Failed to initialize workout_execution test database pool: %v", err)
	}

	db, err := gorm.Open(postgres.New(postgres.Config{
		Conn: sqlDB,
	}), &gorm.Config{
		SkipDefaultTransaction: true,
	})
	if err != nil {
		t.Fatalf("Failed to wrap database in gorm: %v", err)
	}

	ensureTablesExist(db)
	return db
}

func truncateTables(db *gorm.DB) {
	db.Exec("TRUNCATE TABLE workout_execution.rep_logs CASCADE")
	db.Exec("TRUNCATE TABLE workout_execution.workout_set_logs CASCADE")
	db.Exec("TRUNCATE TABLE workout_execution.session_errors CASCADE")
	db.Exec("TRUNCATE TABLE workout_execution.workout_sessions CASCADE")
	db.Exec("TRUNCATE TABLE workout_execution.personal_records CASCADE")
	db.Exec("TRUNCATE TABLE workout_execution.motion_specifications CASCADE")
	db.Exec("TRUNCATE TABLE workout_execution.outbox CASCADE")
	db.Exec("TRUNCATE TABLE workout_execution.outbox_log CASCADE")
}
