//go:build e2e

package e2e

import (
	"context"
	"testing"

	"google.golang.org/grpc/metadata"

	exercisemsg "github.com/viethung213/gym-companion/internal/gen/go/contracts/supporting/exercise/v1/message"
)

func TestExercise_Lifecycle_E2E(t *testing.T) {
	suite := SetupE2ESuite(t)
	defer suite.StopServer()

	adminCtx := metadata.NewOutgoingContext(context.Background(), metadata.Pairs("x-user-id", "admin-1", "x-user-role", "Admin"))
	userCtx := metadata.NewOutgoingContext(context.Background(), metadata.Pairs("x-user-id", "user-1", "x-user-role", "User"))

	// 1. Create Exercise (Returns Draft status)
	createResp, err := suite.Client.CreateExercise(adminCtx, &exercisemsg.CreateExerciseRequest{
		Name:           "Bench Press",
		BodyPartId:     "legs",
		EquipmentId:    "barbell",
		TargetMuscleId: "quads",
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
		Id:             exerciseID,
		Name:           "Updated Bench Press",
		BodyPartId:     "legs",
		EquipmentId:    "barbell",
		TargetMuscleId: "quads",
		Difficulty:     "Intermediate",
	})
	if err != nil {
		t.Fatalf("UpdateExercise failed: %v", err)
	}
	if got, want := updateResp.GetExercise().GetName(), "Updated Bench Press"; got != want {
		t.Errorf("Expected name %s, got %s", want, got)
	}

	// 3. Submit for Approval
	submitResp, err := suite.Client.SubmitExerciseForApproval(adminCtx, &exercisemsg.SubmitExerciseForApprovalRequest{
		Id: exerciseID,
	})
	if err != nil {
		t.Fatalf("SubmitExerciseForApproval failed: %v", err)
	}
	if got, want := submitResp.GetExercise().GetStatus(), exercisemsg.ExerciseStatus_EXERCISE_STATUS_PENDING_APPROVAL; got != want {
		t.Errorf("Expected status %v, got %v", want, got)
	}

	// 4. Approve Exercise (status: ACTIVE)
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
	if got, want := searchResp.GetExercises()[0].GetId(), exerciseID; got != want {
		t.Errorf("Expected exercise ID %s, got %s", want, got)
	}

	// 6. Get Catalog Metadata
	metadataResp, err := suite.Client.GetCatalogMetadata(userCtx, &exercisemsg.GetCatalogMetadataRequest{})
	if err != nil {
		t.Fatalf("GetCatalogMetadata failed: %v", err)
	}
	if len(metadataResp.GetBodyParts()) == 0 {
		t.Errorf("Expected non-empty body parts metadata")
	}

	// 7. Archive Exercise (returns DeleteExerciseResponse)
	deleteResp, err := suite.Client.DeleteExercise(adminCtx, &exercisemsg.DeleteExerciseRequest{
		Id: exerciseID,
	})
	if err != nil {
		t.Fatalf("DeleteExercise failed: %v", err)
	}
	if !deleteResp.GetSuccess() {
		t.Errorf("Expected delete success = true")
	}

	// 8. Get Archived Exercise as User (should fail / return not found)
	_, err = suite.Client.GetExercise(userCtx, &exercisemsg.GetExerciseRequest{
		Id: exerciseID,
	})
	if err == nil {
		t.Fatalf("Expected error when user tries to get archived exercise, got nil")
	}
}
