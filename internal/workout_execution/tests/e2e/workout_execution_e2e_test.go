//go:build e2e || integration

package e2e

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	workoutexecutionv1message "github.com/viethung213/gym-companion/internal/gen/go/contracts/core/workout_execution/v1/message"
	"google.golang.org/grpc/metadata"
	"google.golang.org/protobuf/types/known/timestamppb"
)

func TestWorkoutExecution_FullLifecycle_E2E(t *testing.T) {
	suite := SetupE2ESuite(t)
	defer suite.StopServer()

	// 1. Seed Mock Data
	seededExerciseID := SeedMockData(t, suite.DB)
	defer CleanMockData(suite.DB)

	userID := uuid.NewString()
	planID := uuid.NewString()
	ctx := metadata.NewOutgoingContext(context.Background(), metadata.Pairs("x-user-id", userID))

	// 2. Query Seeded Motion Specification
	motionResp, err := suite.Client.GetMotionSpecification(ctx, &workoutexecutionv1message.GetMotionSpecificationRequest{
		ExerciseId:       seededExerciseID,
		CoachPersonality: "coach-pro",
	})
	if err != nil {
		t.Fatalf("GetMotionSpecification for seeded mock data failed: %v", err)
	}
	if motionResp.GetOnnxDetectorUrl() == "" || motionResp.GetDialogueEngineUrl() == "" {
		t.Errorf("Unexpected GetMotionSpecification response: %+v", motionResp)
	}

	// 3. Query Seeded Personal Record
	ctxSeed := metadata.NewOutgoingContext(context.Background(), metadata.Pairs("x-user-id", "user-test-seed"))
	prResp, err := suite.Client.GetPersonalRecords(ctxSeed, &workoutexecutionv1message.GetPersonalRecordsRequest{
		ExerciseIds: []string{seededExerciseID},
	})
	if err != nil || len(prResp.GetRecords()) != 1 {
		t.Fatalf("GetPersonalRecords for seeded mock data failed: err=%v, len=%d", err, len(prResp.GetRecords()))
	}
	if prResp.GetRecords()[0].GetOneRepMax() != 60.0 {
		t.Errorf("got OneRepMax = %v, want 60.0", prResp.GetRecords()[0].GetOneRepMax())
	}

	// 4. Start Workout Session
	startResp, err := suite.Client.StartWorkoutSession(ctx, &workoutexecutionv1message.StartWorkoutSessionRequest{
		PlanId: planID,
	})
	if err != nil {
		t.Fatalf("StartWorkoutSession failed: %v", err)
	}
	sessionID := startResp.GetSessionId()
	if sessionID == "" {
		t.Fatalf("StartWorkoutSession returned empty SessionId")
	}

	// 5. Duplicate Start Workout Session (should fail)
	_, err = suite.Client.StartWorkoutSession(ctx, &workoutexecutionv1message.StartWorkoutSessionRequest{
		PlanId: planID,
	})
	if err == nil {
		t.Fatalf("Expected error when starting duplicate active session, got nil")
	}

	// 6. Log Set 1 (AI Set with RepLogs)
	formScore := float32(95.0)
	logSetResp, err := suite.Client.LogWorkoutSet(ctx, &workoutexecutionv1message.LogWorkoutSetRequest{
		SessionId:   sessionID,
		SetNumber:   1,
		ExerciseId:  seededExerciseID,
		TargetReps:  10,
		ActualReps:  10,
		Weight:      90.0,
		FormScore:   &formScore,
		Rpe:         8.0,
		CameraAngle: "front",
		Reps: []*workoutexecutionv1message.RepLog{
			{RepNumber: 1, RomPercentage: 95.0},
			{RepNumber: 2, RomPercentage: 92.0},
		},
	})
	if err != nil || logSetResp.GetSetLogId() == "" {
		t.Fatalf("LogWorkoutSet failed: err=%v, resp=%v", err, logSetResp)
	}

	// 7. Sync Posture Errors
	now := timestamppb.New(time.Now().UTC())
	_, err = suite.Client.SyncWorkoutLogs(ctx, &workoutexecutionv1message.SyncWorkoutLogsRequest{
		SessionId: sessionID,
		Errors: []*workoutexecutionv1message.ErrorLog{
			{
				ErrorCode:  "ERR_KNEE_OVER_TOE",
				Severity:   "MEDIUM",
				Timestamp:  now,
				SetNumber:  1,
				RepNumber:  2,
				ExerciseId: seededExerciseID,
			},
		},
	})
	if err != nil {
		t.Fatalf("SyncWorkoutLogs failed: %v", err)
	}

	// 8. Get Workout Session Errors
	errsResp, err := suite.Client.GetWorkoutSessionErrors(ctx, &workoutexecutionv1message.GetWorkoutSessionErrorsRequest{
		SessionId: sessionID,
	})
	if err != nil || len(errsResp.GetErrors()) != 1 {
		t.Fatalf("GetWorkoutSessionErrors failed: err=%v, len=%d", err, len(errsResp.GetErrors()))
	}

	// 9. Complete Workout Session
	completeResp, err := suite.Client.CompleteWorkoutSession(ctx, &workoutexecutionv1message.CompleteWorkoutSessionRequest{
		SessionId: sessionID,
	})
	if err != nil || completeResp.GetSessionId() != sessionID {
		t.Fatalf("CompleteWorkoutSession failed: err=%v, resp=%v", err, completeResp)
	}
	if completeResp.GetTotalSets() != 1 || completeResp.GetTotalVolume() != 900.0 {
		t.Errorf("Unexpected summary stats: sets=%d, volume=%f", completeResp.GetTotalSets(), completeResp.GetTotalVolume())
	}

	// 10. Get Workout History
	historyResp, err := suite.Client.GetWorkoutHistory(ctx, &workoutexecutionv1message.GetWorkoutHistoryRequest{
		Limit:  10,
		Offset: 0,
	})
	if err != nil || len(historyResp.GetSessions()) != 1 {
		t.Fatalf("GetWorkoutHistory failed: err=%v, len=%d", err, len(historyResp.GetSessions()))
	}

	// 11. Test Abort Flow on a New Session
	userID2 := uuid.NewString()
	ctx2 := metadata.NewOutgoingContext(context.Background(), metadata.Pairs("x-user-id", userID2))
	start2, err := suite.Client.StartWorkoutSession(ctx2, &workoutexecutionv1message.StartWorkoutSessionRequest{
		PlanId: planID,
	})
	if err != nil {
		t.Fatalf("StartWorkoutSession 2 failed: %v", err)
	}

	abortResp, err := suite.Client.AbortWorkoutSession(ctx2, &workoutexecutionv1message.AbortWorkoutSessionRequest{
		SessionId: start2.GetSessionId(),
		Reason:    "User requested stop",
	})
	if err != nil || abortResp.GetSessionId() != start2.GetSessionId() {
		t.Fatalf("AbortWorkoutSession failed: err=%v, resp=%v", err, abortResp)
	}
}
