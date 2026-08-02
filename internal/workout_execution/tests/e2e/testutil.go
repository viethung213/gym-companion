//go:build e2e || integration

package e2e

import (
	"context"
	"net"
	"testing"
	"time"

	"github.com/google/uuid"
	workoutexecutionv1service "github.com/viethung213/gym-companion/internal/gen/go/contracts/core/workout_execution/v1/service"
	"github.com/viethung213/gym-companion/internal/shared/database"
	workoutexecution "github.com/viethung213/gym-companion/internal/workout_execution"
	infraPostgres "github.com/viethung213/gym-companion/internal/workout_execution/infrastructure/persistence"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

type E2ETestSuite struct {
	DB         *gorm.DB
	GRPCConn   *grpc.ClientConn
	Client     workoutexecutionv1service.WorkoutExecutionServiceClient
	StopServer func()
}

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
		version INT NOT NULL DEFAULT 1
	);`)
	db.Exec(`ALTER TABLE workout_execution.workout_sessions ADD COLUMN IF NOT EXISTS version INT NOT NULL DEFAULT 1;`)

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

	db.Exec(`CREATE TABLE IF NOT EXISTS workout_execution.motion_specifications (
		exercise_id VARCHAR(255) PRIMARY KEY,
		onnx_detector_url VARCHAR(1024),
		onnx_skeleton_url VARCHAR(1024),
		local_rules_url VARCHAR(1024),
		dialogue_engine_url VARCHAR(1024),
		recommended_camera_angle VARCHAR(50),
		is_ready BOOLEAN NOT NULL DEFAULT FALSE,
		created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
		updated_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP
	);`)

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

// CleanMockData truncates all tables in workout_execution schema to clean up state.
func CleanMockData(db *gorm.DB) {
	db.Exec("TRUNCATE TABLE workout_execution.rep_logs CASCADE")
	db.Exec("TRUNCATE TABLE workout_execution.workout_set_logs CASCADE")
	db.Exec("TRUNCATE TABLE workout_execution.session_errors CASCADE")
	db.Exec("TRUNCATE TABLE workout_execution.workout_sessions CASCADE")
	db.Exec("TRUNCATE TABLE workout_execution.personal_records CASCADE")
	db.Exec("TRUNCATE TABLE workout_execution.motion_specifications CASCADE")
	db.Exec("TRUNCATE TABLE workout_execution.outbox CASCADE")
	db.Exec("TRUNCATE TABLE workout_execution.outbox_log CASCADE")
}

// SeedMockData seeds initial mock data into database for testing.
func SeedMockData(t *testing.T, db *gorm.DB) string {
	t.Helper()

	exerciseID := "ex-bench-press"
	now := time.Now().UTC()

	// 1. Seed Motion Specification
	motionModel := infraPostgres.MotionSpecificationModel{
		ExerciseID:             exerciseID,
		OnnxDetectorURL:        "http://storage.fitai.com/models/detector.onnx",
		OnnxSkeletonURL:        "http://storage.fitai.com/models/skeleton.onnx",
		LocalRulesURL:          "http://storage.fitai.com/rules/bench_press.json",
		DialogueEngineURL:      "http://storage.fitai.com/dialogue/coach-pro.json",
		RecommendedCameraAngle: "front",
		CreatedAt:              now,
		UpdatedAt:              now,
	}
	if err := db.Save(&motionModel).Error; err != nil {
		t.Fatalf("Failed to seed MotionSpecification: %v", err)
	}

	// 2. Seed Personal Record
	prModel := infraPostgres.PersonalRecordModel{
		ID:           uuid.NewString(),
		UserID:       "user-test-seed",
		ExerciseID:   exerciseID,
		OneRepMax:    60.0,
		Weight:       60.0,
		Reps:         1,
		FormVerified: true,
		AchievedAt:   now,
		CreatedAt:    now,
		UpdatedAt:    now,
	}
	if err := db.Save(&prModel).Error; err != nil {
		t.Fatalf("Failed to seed PersonalRecord: %v", err)
	}

	// 3. Seed Outbox Record
	outboxModel := infraPostgres.OutboxModel{
		ID:            uuid.NewString(),
		EventID:       uuid.NewString(),
		AggregateType: "WorkoutSession",
		AggregateID:   "sess-seed-1",
		EventType:     "contracts.core.workout_execution.v1.workoutSessionStarted",
		Payload:       []byte(`{"sessionId":"sess-seed-1"}`),
		PartitionKey:  "sess-seed-1",
		Published:     false,
		CreatedAt:     now,
	}
	if err := db.Save(&outboxModel).Error; err != nil {
		t.Fatalf("Failed to seed OutboxModel: %v", err)
	}

	return exerciseID
}

func SetupE2ESuite(t *testing.T) *E2ETestSuite {
	t.Helper()

	sqlDB, err := database.GetRegistry().GetPool("workout_execution")
	if err != nil {
		t.Fatalf("Failed to initialize workout_execution DB pool: %v", err)
	}

	db, err := gorm.Open(postgres.New(postgres.Config{
		Conn: sqlDB,
	}), &gorm.Config{
		SkipDefaultTransaction: true,
	})
	if err != nil {
		t.Fatalf("Failed to wrap database in gorm: %v", err)
	}

	// Ensure tables exist & clean initial state
	ensureTablesExist(db)
	CleanMockData(db)

	// Start local gRPC server on a random free port
	lis, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("Failed to listen on random port: %v", err)
	}

	grpcServer := grpc.NewServer()
	ctx, cancel := context.WithCancel(context.Background())

	cleanup, err := workoutexecution.Initialize(ctx, workoutexecution.ModuleDeps{
		DB:         sqlDB,
		GRPCServer: grpcServer,
	})
	if err != nil {
		cancel()
		t.Fatalf("Failed to initialize workout_execution module: %v", err)
	}

	go func() {
		_ = grpcServer.Serve(lis)
	}()

	conn, err := grpc.NewClient(lis.Addr().String(), grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		grpcServer.Stop()
		cleanup()
		cancel()
		t.Fatalf("Failed to connect to gRPC test server: %v", err)
	}

	client := workoutexecutionv1service.NewWorkoutExecutionServiceClient(conn)

	stopServer := func() {
		// Clean mock data after tests complete
		CleanMockData(db)
		_ = conn.Close()
		grpcServer.GracefulStop()
		cleanup()
		cancel()
	}

	return &E2ETestSuite{
		DB:         db,
		GRPCConn:   conn,
		Client:     client,
		StopServer: stopServer,
	}
}
