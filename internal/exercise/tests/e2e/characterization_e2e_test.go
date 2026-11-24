//go:build e2e

package e2e

import (
	"context"
	"testing"

	exercisemsg "github.com/viethung213/gym-companion/internal/gen/go/contracts/supporting/exercise/v1/message"
	"google.golang.org/grpc/metadata"
)

// TestCharacterization_F1_SearchExcludesMuscles pins finding F1 of
// docs/05_exercise_catalog_refactor_spec.md §3: SearchExercises used to drop
// avoid_injury_areas, so a search avoiding the quadriceps still returned a
// squat. exclude_muscles (§8.2 item 4) must drop exercises that load an
// excluded muscle as primary or secondary and keep the rest.
func TestCharacterization_F1_SearchExcludesMuscles(t *testing.T) {
	suite := SetupE2ESuite(t)
	defer suite.StopServer()

	adminCtx := metadata.NewOutgoingContext(context.Background(),
		metadata.Pairs("x-user-id", "admin-1", "x-user-role", "Admin"))
	userCtx := metadata.NewOutgoingContext(context.Background(),
		metadata.Pairs("x-user-id", "user-1", "x-user-role", "User"))

	squatID := createActiveExercise(t, suite, adminCtx, "F1 Barbell Squat",
		exercisemsg.MuscleGroup_MUSCLE_GROUP_QUADRICEPS, exercisemsg.MuscleGroup_MUSCLE_GROUP_GLUTES)
	lungeID := createActiveExercise(t, suite, adminCtx, "F1 Step Up",
		exercisemsg.MuscleGroup_MUSCLE_GROUP_GLUTES, exercisemsg.MuscleGroup_MUSCLE_GROUP_QUADRICEPS)
	hipThrustID := createActiveExercise(t, suite, adminCtx, "F1 Hip Thrust",
		exercisemsg.MuscleGroup_MUSCLE_GROUP_GLUTES, exercisemsg.MuscleGroup_MUSCLE_GROUP_HAMSTRINGS)

	resp, err := suite.Client.SearchExercises(userCtx, &exercisemsg.SearchExercisesRequest{
		Keyword:        "F1",
		ExcludeMuscles: []exercisemsg.MuscleGroup{exercisemsg.MuscleGroup_MUSCLE_GROUP_QUADRICEPS},
	})
	if err != nil {
		t.Fatalf("SearchExercises failed: %v", err)
	}

	gotIDs := make(map[string]bool, len(resp.GetExercises()))
	for _, ex := range resp.GetExercises() {
		gotIDs[ex.GetId()] = true
	}

	if gotIDs[squatID] {
		t.Errorf("got squat %s (primary QUADRICEPS), want it excluded", squatID)
	}
	if gotIDs[lungeID] {
		t.Errorf("got step up %s (secondary QUADRICEPS), want it excluded", lungeID)
	}
	if !gotIDs[hipThrustID] || len(gotIDs) != 1 {
		t.Errorf("got results %v, want only hip thrust %s", gotIDs, hipThrustID)
	}
}

// createActiveExercise creates a barbell strength exercise and walks it to
// ACTIVE so SearchExercises returns it.
func createActiveExercise(
	t *testing.T,
	suite *TestSuite,
	adminCtx context.Context,
	name string,
	primary, secondary exercisemsg.MuscleGroup,
) string {
	t.Helper()

	createResp, err := suite.Client.CreateExercise(adminCtx, &exercisemsg.CreateExerciseRequest{
		Name:             name,
		PrimaryMuscle:    primary,
		SecondaryMuscles: []exercisemsg.MuscleGroup{secondary},
		Equipment:        exercisemsg.Equipment_EQUIPMENT_BARBELL,
		Category:         exercisemsg.ExerciseCategory_EXERCISE_CATEGORY_STRENGTH,
		Level:            exercisemsg.ExerciseLevel_EXERCISE_LEVEL_BEGINNER,
	})
	if err != nil {
		t.Fatalf("CreateExercise %q failed: %v", name, err)
	}

	id := createResp.GetExercise().GetId()
	if _, err := suite.Client.ApproveExercise(adminCtx,
		&exercisemsg.ApproveExerciseRequest{Id: id}); err != nil {
		t.Fatalf("ApproveExercise %q failed: %v", name, err)
	}

	return id
}
