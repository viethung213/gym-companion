package ai_test

import (
	"context"
	"testing"
	"time"

	"github.com/viethung213/gym-companion/internal/coaching/application/port"
	coachingai "github.com/viethung213/gym-companion/internal/coaching/infrastructure/ai"
)

func TestGeminiCoachAgent(t *testing.T) {
	t.Parallel()
	agent := coachingai.NewGeminiCoachAgent("mock_api_key")
	ctx := context.Background()

	t.Run("GenerateRoadmapStrategy", func(t *testing.T) {
		t.Parallel()
		out, err := agent.GenerateRoadmapStrategy(ctx, port.RoadmapStrategyParams{
			UserID:          "usr_100",
			Goal:            "Hypertrophy",
			ExperienceLevel: "Intermediate",
		})
		if err != nil {
			t.Fatalf("expected no error, got %v", err)
		}
		if out.StartDate.IsZero() || out.EndDate.IsZero() {
			t.Errorf("expected non-zero start/end dates")
		}
	})

	t.Run("GenerateWeeklySchedule", func(t *testing.T) {
		t.Parallel()
		out, err := agent.GenerateWeeklySchedule(ctx, port.WeeklyScheduleParams{
			UserID:     "usr_100",
			RoadmapID:  "rdp_1",
			WeekNumber: 1,
			Goal:       "Hypertrophy",
		})
		if err != nil {
			t.Fatalf("expected no error, got %v", err)
		}
		if len(out.Days) != 7 {
			t.Errorf("expected 7 schedule days, got %d", len(out.Days))
		}
	})

	t.Run("GenerateDailyPrescription", func(t *testing.T) {
		t.Parallel()
		out, err := agent.GenerateDailyPrescription(ctx, port.DailyPrescriptionParams{
			UserID:                  "usr_100",
			RoadmapID:               "rdp_1",
			WeeklyScheduleID:        "ws_1",
			ScheduledDate:           time.Now().UTC(),
			TargetMuscleGroups:      []string{"Chest"},
			AvailableEquipment:      []string{"Barbell"},
			AnomalousSessionDetected: true,
			IsDeloadWeek:            true,
		})
		if err != nil {
			t.Fatalf("expected no error, got %v", err)
		}
		if len(out.Prescription.MainExercises()) == 0 {
			t.Errorf("expected main exercises in prescription")
		}
		if out.Prescription.MainExercises()[0].TargetRPE() > 6.0 {
			t.Errorf("expected Deload RPE <= 6.0, got %f", out.Prescription.MainExercises()[0].TargetRPE())
		}
	})
}
