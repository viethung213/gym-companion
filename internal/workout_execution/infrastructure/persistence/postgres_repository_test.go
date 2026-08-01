//go:build integration

package persistence_test

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/viethung213/gym-companion/internal/shared/database"
	"github.com/viethung213/gym-companion/internal/workout_execution/application/port"
	"github.com/viethung213/gym-companion/internal/workout_execution/domain/aggregate"
	"github.com/viethung213/gym-companion/internal/workout_execution/domain/derror"
	"github.com/viethung213/gym-companion/internal/workout_execution/domain/vo"
	infraPostgres "github.com/viethung213/gym-companion/internal/workout_execution/infrastructure/persistence"
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
		updated_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP
	);`)
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
}

func TestPostgresWorkoutSessionRepository_Integration(t *testing.T) {
	db := getTestDB(t)
	truncateTables(db)
	defer truncateTables(db)

	repo := infraPostgres.NewPostgresWorkoutSessionRepository(db)
	ctx := context.Background()

	sessID := uuid.NewString()
	userID := uuid.NewString()
	planID := uuid.NewString()

	// 1. Create and Save Session
	session, err := aggregate.NewWorkoutSession(sessID, userID, planID)
	if err != nil {
		t.Fatalf("Failed to create session: %v", err)
	}

	formScore := float32(88.0)
	rep := vo.NewRepLog(1, 90.0, []string{"ERR_BACK"}, map[string]float32{"knee": 100.0})
	err = session.LogSet(aggregate.WorkoutSetLog{
		ID:          uuid.NewString(),
		SetNumber:   1,
		ExerciseID:  "ex-bench",
		TargetReps:  10,
		ActualReps:  10,
		Weight:      70.0,
		FormScore:   &formScore,
		RPE:         8.0,
		CameraAngle: "front",
		Reps:        []vo.RepLog{rep},
	})
	if err != nil {
		t.Fatalf("Failed to log set: %v", err)
	}

	session.AddErrors([]aggregate.SessionError{
		{
			ID:         uuid.NewString(),
			SetNumber:  1,
			RepNumber:  1,
			ExerciseID: "ex-bench",
			ErrorCode:  "ERR_BACK",
			Severity:   "MEDIUM",
			Timestamp:  time.Now().UTC(),
		},
	})

	if err := repo.Save(ctx, session); err != nil {
		t.Fatalf("Failed to save session: %v", err)
	}

	// 2. FindByID (success and non-existent)
	found, err := repo.FindByID(ctx, sessID)
	if err != nil || found == nil {
		t.Fatalf("FindByID failed: err=%v, found=%v", err, found)
	}
	if found.ID() != sessID || len(found.Sets()) != 1 || len(found.Errors()) != 1 {
		t.Errorf("Mismatch in retrieved session fields: ID=%s, sets=%d, errors=%d", found.ID(), len(found.Sets()), len(found.Errors()))
	}

	missingSession, err := repo.FindByID(ctx, "non-existent-id")
	if err != nil || missingSession != nil {
		t.Errorf("expected nil, nil for missing session FindByID, got %v, %v", missingSession, err)
	}

	// 3. FindActiveSessionByUserID (success and non-existent)
	active, err := repo.FindActiveSessionByUserID(ctx, userID)
	if err != nil || active == nil {
		t.Fatalf("FindActiveSessionByUserID failed: err=%v, active=%v", err, active)
	}

	missingActive, err := repo.FindActiveSessionByUserID(ctx, "non-existent-user")
	if err != nil || missingActive != nil {
		t.Errorf("expected nil, nil for missing active session, got %v, %v", missingActive, err)
	}

	// 4. FindTimedOutSessions
	timedOut, err := repo.FindTimedOutSessions(ctx, 240)
	if err != nil {
		t.Fatalf("FindTimedOutSessions failed: %v", err)
	}
	_ = timedOut

	// 5. GetRecentVolumesForMuscleGroup
	vols, err := repo.GetRecentVolumesForMuscleGroup(ctx, userID, "Chest", 5)
	if err != nil {
		t.Fatalf("GetRecentVolumesForMuscleGroup failed: %v", err)
	}
	_ = vols

	// 6. Complete session & FindHistoryByUserID
	_ = session.Complete(false, false)
	if err := repo.Save(ctx, session); err != nil {
		t.Fatalf("Failed to save completed session: %v", err)
	}

	history, err := repo.FindHistoryByUserID(ctx, userID, 10, 0)
	if err != nil || len(history) != 1 {
		t.Errorf("FindHistoryByUserID failed: err=%v, len=%d", err, len(history))
	}
}

func TestPostgresPersonalRecordRepository_Integration(t *testing.T) {
	db := getTestDB(t)
	truncateTables(db)
	defer truncateTables(db)

	repo := infraPostgres.NewPostgresPersonalRecordRepository(db)
	ctx := context.Background()

	userID := uuid.NewString()
	exID := "ex-squat"
	now := time.Now().UTC()

	pr := aggregate.NewPersonalRecord(uuid.NewString(), userID, exID, 120.0, 5, true, now)
	if err := repo.Save(ctx, pr); err != nil {
		t.Fatalf("Failed to save PR: %v", err)
	}

	// FindByUserIDAndExerciseID (success and non-existent)
	found, err := repo.FindByUserIDAndExerciseID(ctx, userID, exID)
	if err != nil || found == nil {
		t.Fatalf("FindByUserIDAndExerciseID failed: err=%v, found=%v", err, found)
	}
	if found.Weight() != 120.0 {
		t.Errorf("got weight = %v, want 120.0", found.Weight())
	}

	missingPR, err := repo.FindByUserIDAndExerciseID(ctx, userID, "non-existent-ex")
	if err != nil || missingPR != nil {
		t.Errorf("expected nil, nil for missing PR, got %v, %v", missingPR, err)
	}

	// FindByUserIDAndExerciseIDs
	list, err := repo.FindByUserIDAndExerciseIDs(ctx, userID, []string{exID})
	if err != nil || len(list) != 1 {
		t.Errorf("FindByUserIDAndExerciseIDs failed: err=%v, len=%d", err, len(list))
	}
}

func TestPostgresMotionSpecificationRepository_Integration(t *testing.T) {
	db := getTestDB(t)
	truncateTables(db)
	defer truncateTables(db)

	repo := infraPostgres.NewPostgresMotionSpecificationRepository(db)
	ctx := context.Background()

	exID := "ex-deadlift"
	spec := aggregate.RestoreMotionSpecification(exID, "http://detector.onnx", "http://skeleton.onnx", "http://rules.json", "http://dialogue.json", "side", true, time.Now().UTC(), time.Now().UTC())

	if err := repo.Save(ctx, spec); err != nil {
		t.Fatalf("Failed to save MotionSpec: %v", err)
	}

	found, err := repo.FindByExerciseID(ctx, exID)
	if err != nil || found == nil {
		t.Fatalf("FindByExerciseID failed: err=%v, found=%v", err, found)
	}
	if found.OnnxDetectorURL() != "http://detector.onnx" {
		t.Errorf("got detector URL = %v, want http://detector.onnx", found.OnnxDetectorURL())
	}

	missingSpec, err := repo.FindByExerciseID(ctx, "non-existent-ex")
	if !errors.Is(err, derror.ErrNotFound) || missingSpec != nil {
		t.Errorf("expected derror.ErrNotFound for missing MotionSpec, got %v, %v", missingSpec, err)
	}
}

func TestPostgresOutboxRepository_Integration(t *testing.T) {
	db := getTestDB(t)
	truncateTables(db)
	defer truncateTables(db)

	repo := infraPostgres.NewPostgresOutboxRepository(db)
	ctx := context.Background()

	evID := uuid.NewString()
	record := &port.OutboxRecord{
		ID:            evID,
		AggregateType: "WorkoutSession",
		AggregateID:   "sess-1",
		EventType:     "WorkoutSessionStarted",
		Payload:       []byte(`{"test":true}`),
		Published:     false,
		CreatedAt:     time.Now().UTC(),
	}

	if err := repo.Save(ctx, record); err != nil {
		t.Fatalf("Failed to save OutboxRecord: %v", err)
	}

	unpublished, err := repo.FetchUnpublished(ctx, 10)
	if err != nil || len(unpublished) != 1 {
		t.Fatalf("FetchUnpublished failed: err=%v, len=%d", err, len(unpublished))
	}

	// MarkPublished with empty slice (len==0)
	if err := repo.MarkPublished(ctx, []string{}); err != nil {
		t.Fatalf("MarkPublished empty slice failed: %v", err)
	}

	if err := repo.MarkPublished(ctx, []string{evID}); err != nil {
		t.Fatalf("MarkPublished failed: %v", err)
	}

	// ProcessBatch test
	err = repo.ProcessBatch(ctx, 10, func(txCtx context.Context, records []*port.OutboxRecord) error {
		return nil
	})
	if err != nil {
		t.Fatalf("ProcessBatch failed: %v", err)
	}
}

func TestSQLTransactionManager_Integration(t *testing.T) {
	db := getTestDB(t)
	truncateTables(db)
	defer truncateTables(db)

	tm := infraPostgres.NewSQLTransactionManager(db)
	repo := infraPostgres.NewPostgresWorkoutSessionRepository(db)
	ctx := context.Background()

	sessID := uuid.NewString()
	session, _ := aggregate.NewWorkoutSession(sessID, "u1", "p1")

	// 1. Transaction Commit
	err := tm.WithTransaction(ctx, func(txCtx context.Context) error {
		return repo.Save(txCtx, session)
	})
	if err != nil {
		t.Fatalf("WithTransaction commit failed: %v", err)
	}

	// 2. Transaction Rollback on error
	errRollback := errors.New("force rollback")
	err = tm.WithTransaction(ctx, func(txCtx context.Context) error {
		return errRollback
	})
	if !errors.Is(err, errRollback) {
		t.Errorf("got %v, want %v", err, errRollback)
	}

	// 3. Transaction Commit error (canceled context inside transaction)
	err = tm.WithTransaction(ctx, func(txCtx context.Context) error {
		ctxCancel, cancel := context.WithCancel(txCtx)
		cancel()
		_ = repo.Save(ctxCancel, session)
		return nil
	})
	_ = err

	// Verify saved session still exists
	found, err := repo.FindByID(ctx, sessID)
	if err != nil || found == nil {
		t.Fatalf("Failed to find committed session")
	}
}

func TestPostgresRepository_CanceledContextAndLockConflict(t *testing.T) {
	db := getTestDB(t)
	truncateTables(db)
	defer truncateTables(db)

	ctxCancel, cancel := context.WithCancel(context.Background())
	cancel() // Canceled context simulates database connection loss / I/O timeout

	sessionRepo := infraPostgres.NewPostgresWorkoutSessionRepository(db)
	prRepo := infraPostgres.NewPostgresPersonalRecordRepository(db)
	motionRepo := infraPostgres.NewPostgresMotionSpecificationRepository(db)
	outboxRepo := infraPostgres.NewPostgresOutboxRepository(db)

	// 1. Canceled context errors (DB connection lost / canceled context)
	session, _ := aggregate.NewWorkoutSession("s1", "u1", "p1")
	if err := sessionRepo.Save(ctxCancel, session); err == nil {
		t.Error("expected error on canceled context Save session")
	}
	if _, err := sessionRepo.FindByID(ctxCancel, "s1"); err == nil {
		t.Error("expected error on canceled context FindByID session")
	}
	if _, err := sessionRepo.FindActiveSessionByUserID(ctxCancel, "u1"); err == nil {
		t.Error("expected error on canceled context FindActiveSessionByUserID")
	}
	if _, err := sessionRepo.FindTimedOutSessions(ctxCancel, 240); err == nil {
		t.Error("expected error on canceled context FindTimedOutSessions")
	}
	if _, err := sessionRepo.FindHistoryByUserID(ctxCancel, "u1", 10, 0); err == nil {
		t.Error("expected error on canceled context FindHistoryByUserID")
	}
	if _, err := sessionRepo.GetRecentVolumesForMuscleGroup(ctxCancel, "u1", "Chest", 5); err == nil {
		t.Error("expected error on canceled context GetRecentVolumesForMuscleGroup")
	}

	pr := aggregate.NewPersonalRecord("pr1", "u1", "ex1", 100, 10, true, time.Now().UTC())
	if err := prRepo.Save(ctxCancel, pr); err == nil {
		t.Error("expected error on canceled context Save PR")
	}
	if _, err := prRepo.FindByUserIDAndExerciseID(ctxCancel, "u1", "ex1"); err == nil {
		t.Error("expected error on canceled context FindByUserIDAndExerciseID")
	}
	if _, err := prRepo.FindByUserIDAndExerciseIDs(ctxCancel, "u1", []string{"ex1"}); err == nil {
		t.Error("expected error on canceled context FindByUserIDAndExerciseIDs")
	}

	motionSpec := aggregate.NewDraftMotionSpecification("ex1", "http://detector.onnx", "http://skeleton.onnx")
	if err := motionRepo.Save(ctxCancel, motionSpec); err == nil {
		t.Error("expected error on canceled context Save MotionSpec")
	}
	if _, err := motionRepo.FindByExerciseID(ctxCancel, "ex1"); err == nil {
		t.Error("expected error on canceled context FindByExerciseID")
	}

	outboxRecord := &port.OutboxRecord{ID: "o1", EventType: "ev1", Payload: []byte("{}")}
	if err := outboxRepo.Save(ctxCancel, outboxRecord); err == nil {
		t.Error("expected error on canceled context Save Outbox")
	}
	if _, err := outboxRepo.FetchUnpublished(ctxCancel, 10); err == nil {
		t.Error("expected error on canceled context FetchUnpublished")
	}
	if err := outboxRepo.MarkPublished(ctxCancel, []string{"o1"}); err == nil {
		t.Error("expected error on canceled context MarkPublished")
	}

	// 2. ProcessBatch test
	err := outboxRepo.ProcessBatch(context.Background(), 10, func(txCtx context.Context, records []*port.OutboxRecord) error {
		return nil
	})
	if err != nil {
		t.Errorf("ProcessBatch returned unexpected error: %v", err)
	}
}

func TestWorkoutExecutionOutboxRepository_3ConcurrentNodes_Integration(t *testing.T) {
	db := getTestDB(t)
	ensureTablesExist(db)

	repo := infraPostgres.NewPostgresOutboxRepository(db)
	ctx := context.Background()

	const totalRecords = 30
	const batchLimit = 10

	for i := 0; i < totalRecords; i++ {
		id := uuid.New().String()
		rec := &port.OutboxRecord{
			ID:            id,
			EventID:       id,
			AggregateType: "WorkoutSession",
			AggregateID:   fmt.Sprintf("sess-%d", i),
			EventType:     "WorkoutSessionStarted",
			Payload:       []byte(`{}`),
			PartitionKey:  fmt.Sprintf("key-%d", i),
			CreatedAt:     time.Now(),
		}
		err := repo.Save(ctx, rec)
		if err != nil {
			t.Fatalf("Failed to seed record %d: %v", i, err)
		}
	}

	var mu sync.Mutex
	processedIDs := make(map[string]int)

	var wg sync.WaitGroup
	const nodeCount = 3
	wg.Add(nodeCount)

	for nodeIdx := 1; nodeIdx <= nodeCount; nodeIdx++ {
		go func(id int) {
			defer wg.Done()
			for {
				records, err := repo.ClaimBatch(ctx, batchLimit, 5*time.Second)
				if err != nil || len(records) == 0 {
					break
				}

				mu.Lock()
				recIDs := make([]string, len(records))
				for i, r := range records {
					processedIDs[r.ID]++
					recIDs[i] = r.ID
				}
				mu.Unlock()

				time.Sleep(10 * time.Millisecond)
				_ = repo.MarkPublished(ctx, recIDs)
			}
		}(nodeIdx)
	}

	wg.Wait()

	mu.Lock()
	defer mu.Unlock()

	if len(processedIDs) != totalRecords {
		t.Errorf("Expected %d total unique records processed across 3 nodes, got %d", totalRecords, len(processedIDs))
	}

	for id, count := range processedIDs {
		if count > 1 {
			t.Errorf("Record %s was processed %d times (expected exactly 1)", id, count)
		}
	}
}

func TestWorkoutExecutionOutboxRepository_NodeCrashFailover_Integration(t *testing.T) {
	db := getTestDB(t)
	ensureTablesExist(db)

	repo := infraPostgres.NewPostgresOutboxRepository(db)
	ctx := context.Background()

	for i := 0; i < 5; i++ {
		id := uuid.New().String()
		rec := &port.OutboxRecord{
			ID:            id,
			EventID:       id,
			AggregateType: "WorkoutSession",
			AggregateID:   fmt.Sprintf("sess-%d", i),
			EventType:     "WorkoutSessionStarted",
			Payload:       []byte(`{}`),
			PartitionKey:  fmt.Sprintf("key-%d", i),
			CreatedAt:     time.Now(),
		}
		err := repo.Save(ctx, rec)
		if err != nil {
			t.Fatalf("Failed to seed record %d: %v", i, err)
		}
	}

	// Node 1 claims with 100ms lock
	node1Claimed, err := repo.ClaimBatch(ctx, 5, 100*time.Millisecond)
	if err != nil {
		t.Fatalf("Node 1 ClaimBatch failed: %v", err)
	}
	if len(node1Claimed) != 5 {
		t.Fatalf("Node 1 expected 5 claimed records, got %d", len(node1Claimed))
	}

	// Node 1 CRASHES (no MarkPublished)

	// Node 2 tries immediately -> 0 records
	node2Immediate, err := repo.ClaimBatch(ctx, 5, 5*time.Second)
	if err != nil {
		t.Fatalf("Node 2 immediate ClaimBatch failed: %v", err)
	}
	if len(node2Immediate) != 0 {
		t.Errorf("Node 2 expected 0 records while Node 1 lock active, got %d", len(node2Immediate))
	}

	// Wait 150ms for lock expiration
	time.Sleep(150 * time.Millisecond)

	// Node 2 tries after lock expired -> claims all 5
	node2Recovered, err := repo.ClaimBatch(ctx, 5, 5*time.Second)
	if err != nil {
		t.Fatalf("Node 2 failover ClaimBatch failed: %v", err)
	}
	if len(node2Recovered) != 5 {
		t.Errorf("Node 2 expected 5 recovered records after Node 1 crash, got %d", len(node2Recovered))
	}

	recIDs := make([]string, len(node2Recovered))
	for i, r := range node2Recovered {
		recIDs[i] = r.ID
	}
	if err := repo.MarkPublished(ctx, recIDs); err != nil {
		t.Fatalf("Node 2 MarkPublished failed: %v", err)
	}

	unpub, err := repo.FetchUnpublished(ctx, 10)
	if err != nil {
		t.Fatalf("FetchUnpublished failed: %v", err)
	}
	if len(unpub) != 0 {
		t.Errorf("Expected 0 leftover unpublished records, got %d", len(unpub))
	}
}
