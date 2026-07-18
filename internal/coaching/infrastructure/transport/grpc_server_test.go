package transport

import (
	"context"
	"testing"
	"time"

	"github.com/viethung213/gym-companion/internal/coaching/domain"
	coachingmsg "github.com/viethung213/gym-companion/internal/gen/go/contracts/core/coaching/v1/message"
	"github.com/viethung213/gym-companion/internal/shared/middleware"
	datepb "google.golang.org/genproto/googleapis/type/date"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

func TestPlanningInputMapsInlineSnapshot(t *testing.T) {
	t.Parallel()

	input, err := planningInput(&coachingmsg.InitiateRoadmapPayload{
		ProfileSnapshotId: "profile-snapshot-1",
		PlanningProfile: &coachingmsg.PlanningProfileSnapshot{
			Goal:                coachingmsg.PlanningGoal_PLANNING_GOAL_MUSCLE_GAIN,
			ExperienceLevel:     coachingmsg.ExperienceLevel_EXPERIENCE_LEVEL_BEGINNER,
			TrainingDaysPerWeek: 3,
			PreferredWeekdays:   []string{"monday", "Wednesday", "friday"},
			MaxSessionMinutes:   60,
			EquipmentIds:        []string{"bodyweight"},
			Timezone:            "Asia/Ho_Chi_Minh",
			StartDate:           &datepb.Date{Year: 2026, Month: 7, Day: 20},
		},
	})
	if err != nil {
		t.Fatalf("map planning input: %v", err)
	}
	if input.ProfileSnapshotID != "profile-snapshot-1" || input.Goal != domain.PlanningGoalMuscleGain {
		t.Fatalf("unexpected planning input: %#v", input)
	}
	if len(input.PreferredWeekdays) != 3 || input.PreferredWeekdays[1] != time.Wednesday {
		t.Fatalf("unexpected weekdays: %#v", input.PreferredWeekdays)
	}
}

func TestPlanningInputRequiresInlineSnapshotUntilProfileIntegrationExists(t *testing.T) {
	t.Parallel()

	_, err := planningInput(&coachingmsg.InitiateRoadmapPayload{ProfileSnapshotId: "profile-snapshot-1"})
	if status.Code(rpcError(err)) != codes.FailedPrecondition {
		t.Fatalf("expected failed precondition, got %v", rpcError(err))
	}
}

func TestAuthorizeRequestEnforcesOwnership(t *testing.T) {
	t.Parallel()

	ctx := context.WithValue(context.Background(), middleware.UserIDKey, "user-1")
	if err := authorizeRequest(ctx, "user-1"); err != nil {
		t.Fatalf("authorize owner: %v", err)
	}
	if got := status.Code(rpcError(authorizeRequest(ctx, "user-2"))); got != codes.PermissionDenied {
		t.Fatalf("ownership error code = %v, want permission denied", got)
	}
}

func TestToProtoDailyPlanIncludesCompletePrescription(t *testing.T) {
	t.Parallel()

	generatedAt := time.Date(2026, time.July, 20, 8, 0, 0, 0, time.UTC)
	plan := domain.NewDailyWorkoutPlan(
		"plan-1",
		"user-1",
		"roadmap-1",
		"schedule-1",
		generatedAt,
		[]domain.PrescribedExercise{{
			ExerciseID:   "push-up",
			ExerciseName: "Push Up",
			Sets:         3,
			Reps:         10,
			RestSeconds:  75,
		}},
		[]domain.PlannedActivity{{Name: "Warm-up", DurationSeconds: 300}},
		[]domain.PlannedActivity{{Name: "Cool-down", DurationSeconds: 300}},
		generatedAt,
	)

	message := toProtoDailyPlan(plan)
	exercise := message.GetWorkoutPrescription().GetExercises()[0]
	if exercise.GetExerciseName() != "Push Up" || exercise.GetRestSeconds() != 75 {
		t.Fatalf("unexpected exercise mapping: %#v", exercise)
	}
	if len(message.GetWorkoutPrescription().GetWarmUpItems()) != 1 ||
		len(message.GetWorkoutPrescription().GetCoolDownItems()) != 1 {
		t.Fatal("warm-up and cool-down were not mapped")
	}
}

func TestPageItemsRejectsInvalidToken(t *testing.T) {
	t.Parallel()

	_, _, err := pageItems([]int{1, 2}, 1, "not-an-offset")
	if status.Code(rpcError(err)) != codes.InvalidArgument {
		t.Fatalf("expected invalid argument, got %v", rpcError(err))
	}
}
