//go:build integration

package persistence_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/viethung213/gym-companion/internal/workout_execution/domain/aggregate"
	"github.com/viethung213/gym-companion/internal/workout_execution/domain/derror"
	"github.com/viethung213/gym-companion/internal/workout_execution/domain/vo"
	infraPostgres "github.com/viethung213/gym-companion/internal/workout_execution/infrastructure/persistence"
)

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

	// 2. FindByID & FindByIDForUpdate (success and non-existent)
	found, err := repo.FindByID(ctx, sessID)
	if err != nil || found == nil {
		t.Fatalf("FindByID failed: err=%v, found=%v", err, found)
	}
	if found.ID() != sessID || len(found.Sets()) != 1 || len(found.Errors()) != 1 {
		t.Errorf("Mismatch in retrieved session fields: ID=%s, sets=%d, errors=%d", found.ID(), len(found.Sets()), len(found.Errors()))
	}

	foundForUpdate, err := repo.FindByIDForUpdate(ctx, sessID)
	if err != nil || foundForUpdate == nil {
		t.Fatalf("FindByIDForUpdate failed: err=%v, found=%v", err, foundForUpdate)
	}
	if foundForUpdate.ID() != sessID || len(foundForUpdate.Sets()) != 1 {
		t.Errorf("Mismatch in FindByIDForUpdate fields: ID=%s, sets=%d", foundForUpdate.ID(), len(foundForUpdate.Sets()))
	}

	missingSession, err := repo.FindByID(ctx, "non-existent-id")
	if err != nil || missingSession != nil {
		t.Errorf("expected nil, nil for missing session FindByID, got %v, %v", missingSession, err)
	}

	missingSessionForUpdate, err := repo.FindByIDForUpdate(ctx, "non-existent-id")
	if err != nil || missingSessionForUpdate != nil {
		t.Errorf("expected nil, nil for missing session FindByIDForUpdate, got %v, %v", missingSessionForUpdate, err)
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
	spec := aggregate.RestoreMotionSpecification(exID, "Barbell Deadlift", "http://detector.onnx", "http://skeleton.onnx", "http://rules.json", "http://dialogue.json", "side", true, time.Now().UTC(), time.Now().UTC())

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
	if !errors.Is(err, derror.ErrMotionSpecNotFound) || missingSpec != nil {
		t.Errorf("expected derror.ErrMotionSpecNotFound for missing MotionSpec, got %v, %v", missingSpec, err)
	}

	// Test Search partial substring match in exercise name
	specWithChoice := aggregate.NewDraftMotionSpecification("ex-bench-press", "Barbell Bench Press", "http://detector.onnx", "http://skeleton.onnx")
	if err := repo.Save(ctx, specWithChoice); err != nil {
		t.Fatalf("Failed to save specWithChoice: %v", err)
	}

	// Search with lowercase partial substring "bench" (matches "Barbell Bench Press")
	specs, total, err := repo.Search(ctx, "bench", 10, 0)
	if err != nil {
		t.Fatalf("Search failed: %v", err)
	}
	if total != 1 || len(specs) != 1 {
		t.Errorf("got total=%d, specs count=%d; want 1", total, len(specs))
	} else if specs[0].ExerciseID() != "ex-bench-press" {
		t.Errorf("got exerciseID=%s, want ex-bench-press", specs[0].ExerciseID())
	}

	// Search with keyword matching ONLY an ID
	idOnlySpecs, totalIDOnly, err := repo.Search(ctx, "ex-bench", 10, 0)
	if err != nil {
		t.Fatalf("Search for id-only keyword failed: %v", err)
	}
	if totalIDOnly != 0 || len(idOnlySpecs) != 0 {
		t.Errorf("expected 0 results when searching by ID keyword 'ex-bench', got total=%d, count=%d", totalIDOnly, len(idOnlySpecs))
	}

	// Search with empty keyword returns all specifications
	allSpecs, totalAll, err := repo.Search(ctx, "", 10, 0)
	if err != nil {
		t.Fatalf("Search with empty keyword failed: %v", err)
	}
	if totalAll != 2 || len(allSpecs) != 2 {
		t.Errorf("got totalAll=%d, count=%d; want 2", totalAll, len(allSpecs))
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

func TestPostgresWorkoutSessionRepository_CanceledContext(t *testing.T) {
	db := getTestDB(t)
	truncateTables(db)
	defer truncateTables(db)

	ctxCancel, cancel := context.WithCancel(context.Background())
	cancel() // Canceled context simulates database connection loss / I/O timeout

	sessionRepo := infraPostgres.NewPostgresWorkoutSessionRepository(db)
	prRepo := infraPostgres.NewPostgresPersonalRecordRepository(db)
	motionRepo := infraPostgres.NewPostgresMotionSpecificationRepository(db)

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

	motionSpec := aggregate.NewDraftMotionSpecification("ex1", "Exercise 1", "http://detector.onnx", "http://skeleton.onnx")
	if err := motionRepo.Save(ctxCancel, motionSpec); err == nil {
		t.Error("expected error on canceled context Save MotionSpec")
	}
	if _, err := motionRepo.FindByExerciseID(ctxCancel, "ex1"); err == nil {
		t.Error("expected error on canceled context FindByExerciseID")
	}
}
