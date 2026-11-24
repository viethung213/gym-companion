//go:build e2e

package e2e

import (
	"context"
	"testing"

	exercisemsg "github.com/viethung213/gym-companion/internal/gen/go/contracts/supporting/exercise/v1/message"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
)

func TestExercise_Lifecycle_E2E(t *testing.T) {
	suite := SetupE2ESuite(t)
	defer suite.StopServer()

	adminCtx := metadata.NewOutgoingContext(context.Background(), metadata.Pairs("x-user-id", "admin-1", "x-user-role", "Admin"))
	userCtx := metadata.NewOutgoingContext(context.Background(), metadata.Pairs("x-user-id", "user-1", "x-user-role", "User"))

	// 1. Create Exercise (Returns Draft status)
	createResp, err := suite.Client.CreateExercise(adminCtx, &exercisemsg.CreateExerciseRequest{
		Name:          "Bench Press",
		PrimaryMuscle: exercisemsg.MuscleGroup_MUSCLE_GROUP_CHEST,
		Equipment:     exercisemsg.Equipment_EQUIPMENT_BARBELL,
		Category:      exercisemsg.ExerciseCategory_EXERCISE_CATEGORY_STRENGTH,
		Level:         exercisemsg.ExerciseLevel_EXERCISE_LEVEL_BEGINNER,
	})
	if err != nil {
		t.Fatalf("CreateExercise failed: %v", err)
	}

	exerciseID := createResp.GetExercise().GetId()
	if exerciseID == "" {
		t.Fatalf("CreateExercise returned empty exercise ID")
	}
	if got, want := createResp.GetExercise().GetStatus(), exercisemsg.ExerciseStatus_EXERCISE_STATUS_DRAFT; got != want {
		t.Errorf("Expected status %v, got %v", want, got)
	}

	// 2. Update Exercise
	updateResp, err := suite.Client.UpdateExercise(adminCtx, &exercisemsg.UpdateExerciseRequest{
		Id:            exerciseID,
		Name:          "Updated Bench Press",
		PrimaryMuscle: exercisemsg.MuscleGroup_MUSCLE_GROUP_CHEST,
		Equipment:     exercisemsg.Equipment_EQUIPMENT_BARBELL,
		Category:      exercisemsg.ExerciseCategory_EXERCISE_CATEGORY_STRENGTH,
		Level:         exercisemsg.ExerciseLevel_EXERCISE_LEVEL_INTERMEDIATE,
	})
	if err != nil {
		t.Fatalf("UpdateExercise failed: %v", err)
	}
	if got, want := updateResp.GetExercise().GetName(), "Updated Bench Press"; got != want {
		t.Errorf("Expected name %s, got %s", want, got)
	}
	if got, want := updateResp.GetExercise().GetLevel(), exercisemsg.ExerciseLevel_EXERCISE_LEVEL_INTERMEDIATE; got != want {
		t.Errorf("Expected level %v, got %v", want, got)
	}
	if got, want := updateResp.GetExercise().GetSlug(), "Bench_Press"; got != want {
		t.Errorf("Expected custom slug %s derived from the created name, got %s", want, got)
	}

	// 3. Approve the draft (status: ACTIVE)
	approveResp, err := suite.Client.ApproveExercise(adminCtx, &exercisemsg.ApproveExerciseRequest{
		Id: exerciseID,
	})
	if err != nil {
		t.Fatalf("ApproveExercise failed: %v", err)
	}
	if got, want := approveResp.GetExercise().GetStatus(), exercisemsg.ExerciseStatus_EXERCISE_STATUS_ACTIVE; got != want {
		t.Errorf("Expected status %v, got %v", want, got)
	}

	// 5. Search Active Exercises
	searchResp, err := suite.Client.SearchExercises(userCtx, &exercisemsg.SearchExercisesRequest{
		Keyword: "Updated",
	})
	if err != nil {
		t.Fatalf("SearchExercises failed: %v", err)
	}
	if got := len(searchResp.GetExercises()); got != 1 {
		t.Fatalf("Expected 1 exercise, got %d", got)
	}
	if got, want := searchResp.GetTotalCount(), int32(1); got != want {
		t.Errorf("Expected TotalCount %d, got %d", want, got)
	}
	if got, want := searchResp.GetExercises()[0].GetId(), exerciseID; got != want {
		t.Errorf("Expected exercise ID %s, got %s", want, got)
	}

	// 6. Archive Exercise (returns DeleteExerciseResponse)
	deleteResp, err := suite.Client.DeleteExercise(adminCtx, &exercisemsg.DeleteExerciseRequest{
		Id: exerciseID,
	})
	if err != nil {
		t.Fatalf("DeleteExercise failed: %v", err)
	}
	if !deleteResp.GetSuccess() {
		t.Errorf("Expected delete success = true")
	}

	// 7. Get Archived Exercise as User (should fail / return not found)
	_, err = suite.Client.GetExercise(userCtx, &exercisemsg.GetExerciseRequest{
		Id: exerciseID,
	})
	if err == nil {
		t.Fatalf("Expected error when user tries to get archived exercise, got nil")
	}

	// 8. Restore Exercise as Admin
	restoreResp, err := suite.Client.RestoreExercise(adminCtx, &exercisemsg.RestoreExerciseRequest{
		Id: exerciseID,
	})
	if err != nil {
		t.Fatalf("RestoreExercise failed: %v", err)
	}
	if got, want := restoreResp.GetExercise().GetStatus(), exercisemsg.ExerciseStatus_EXERCISE_STATUS_ACTIVE; got != want {
		t.Errorf("Expected status %v, got %v", want, got)
	}

	// 9. Verify Restored Exercise appears in SearchExercises with TotalCount
	searchAfterRestore, err := suite.Client.SearchExercises(userCtx, &exercisemsg.SearchExercisesRequest{
		Keyword: "Updated",
	})
	if err != nil {
		t.Fatalf("SearchExercises after restore failed: %v", err)
	}
	if got, want := searchAfterRestore.GetTotalCount(), int32(1); got != want {
		t.Errorf("Expected TotalCount %d, got %d", want, got)
	}
	if len(searchAfterRestore.GetExercises()) != 1 {
		t.Fatalf("Expected 1 exercise after restore, got %d", len(searchAfterRestore.GetExercises()))
	}
	if got, want := searchAfterRestore.GetExercises()[0].GetId(), exerciseID; got != want {
		t.Errorf("Expected exercise ID %s, got %s", want, got)
	}
}

func TestCreateExercise_RejectsUnspecifiedTaxonomy_E2E(t *testing.T) {
	suite := SetupE2ESuite(t)
	defer suite.StopServer()

	adminCtx := metadata.NewOutgoingContext(context.Background(), metadata.Pairs("x-user-id", "admin-1", "x-user-role", "Admin"))

	_, err := suite.Client.CreateExercise(adminCtx, &exercisemsg.CreateExerciseRequest{
		Name:          "No Muscle",
		Equipment:     exercisemsg.Equipment_EQUIPMENT_BARBELL,
		Category:      exercisemsg.ExerciseCategory_EXERCISE_CATEGORY_STRENGTH,
		Level:         exercisemsg.ExerciseLevel_EXERCISE_LEVEL_BEGINNER,
		PrimaryMuscle: exercisemsg.MuscleGroup_MUSCLE_GROUP_UNSPECIFIED,
	})
	if got := status.Code(err); got != codes.InvalidArgument {
		t.Errorf("got code %v, want InvalidArgument", got)
	}
}
