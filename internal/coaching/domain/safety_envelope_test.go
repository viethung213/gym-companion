package domain_test

import (
	"testing"
	"time"

	"github.com/viethung213/gym-companion/internal/coaching/domain"
)

func TestUpperSafetyEnvelopeValidator(t *testing.T) {
	validator := domain.NewUpperSafetyEnvelopeValidator()

	t.Run("BR-AC-02 Load Adjustment Ceiling (+30%)", func(t *testing.T) {
		prevWeight := float32(100.0)
		proposedWeight := float32(140.0) // Exceeds +30% (max 130)

		cappedWeight, wasAdjusted := validator.ValidateLoadCeiling(prevWeight, proposedWeight)
		if !wasAdjusted {
			t.Errorf("expected wasAdjusted to be true")
		}
		if cappedWeight != 130.0 {
			t.Errorf("expected cappedWeight to be 130.0, got %f", cappedWeight)
		}
	})

	t.Run("BR-AC-02 Load Adjustment Ceiling (-30%)", func(t *testing.T) {
		prevWeight := float32(100.0)
		proposedWeight := float32(50.0) // Below -30% (min 70)

		cappedWeight, wasAdjusted := validator.ValidateLoadCeiling(prevWeight, proposedWeight)
		if !wasAdjusted {
			t.Errorf("expected wasAdjusted to be true")
		}
		if cappedWeight != 70.0 {
			t.Errorf("expected cappedWeight to be 70.0, got %f", cappedWeight)
		}
	})

	t.Run("BR-AC-02 Load Adjustment Ceiling within range", func(t *testing.T) {
		prevWeight := float32(100.0)
		proposedWeight := float32(115.0)

		cappedWeight, wasAdjusted := validator.ValidateLoadCeiling(prevWeight, proposedWeight)
		if wasAdjusted {
			t.Errorf("expected wasAdjusted to be false")
		}
		if cappedWeight != 115.0 {
			t.Errorf("expected 115.0, got %f", cappedWeight)
		}
	})

	t.Run("Decision 1.4 Deload Week RPE <= 6 lock", func(t *testing.T) {
		exs := []domain.PrescribedExercise{
			domain.NewPrescribedExercise("ex_1", "Bench", 3, 10, 60, 0, "", 90, 120, 8.5),
		}

		// Week 4 (Deload)
		cappedExs, wasAdjusted := validator.ValidateDeloadWeek(4, exs)
		if !wasAdjusted {
			t.Errorf("expected wasAdjusted to be true for Week 4")
		}
		if cappedExs[0].TargetRPE() != 6.0 {
			t.Errorf("expected TargetRPE 6.0, got %f", cappedExs[0].TargetRPE())
		}

		// Week 3 (Non-deload)
		_, wasAdjustedW3 := validator.ValidateDeloadWeek(3, exs)
		if wasAdjustedW3 {
			t.Errorf("expected wasAdjusted to be false for Week 3")
		}
	})

	t.Run("Active Injury Lock pruning", func(t *testing.T) {
		exs := []domain.PrescribedExercise{
			domain.NewPrescribedExercise("ex_1", "Bench Press", 3, 10, 60, 0, "Targets Shoulder and Chest", 90, 120, 7.5),
			domain.NewPrescribedExercise("ex_2", "Squat", 3, 10, 80, 0, "Targets Quads and Glutes", 90, 120, 7.5),
		}

		activeInjuries := []string{"Shoulder"}
		safeExs, prunedCount := validator.ValidateInjuryLocks(exs, activeInjuries)

		if prunedCount != 1 {
			t.Errorf("expected 1 pruned exercise, got %d", prunedCount)
		}
		if len(safeExs) != 1 {
			t.Fatalf("expected 1 safe exercise, got %d", len(safeExs))
		}
		if safeExs[0].ExerciseID() != "ex_2" {
			t.Errorf("expected safe exercise ex_2, got %s", safeExs[0].ExerciseID())
		}
	})

	t.Run("BR-AC-01 Rest Days validation", func(t *testing.T) {
		now := time.Now().UTC()
		days := make([]domain.ScheduleDay, 7)
		for i := 0; i < 7; i++ {
			status := domain.WorkoutDayStatusTraining
			if i == 6 {
				status = domain.WorkoutDayStatusRest
			}
			days[i] = domain.NewScheduleDay(now.AddDate(0, 0, i), "Day", status, []string{"Chest"}, "")
		}
		ws, _ := domain.NewWeeklySchedule("ws_1", "rdp_1", "usr_1", 1, now, now.AddDate(0, 0, 7), "PPL", days)
		err := validator.ValidateRestDays(ws)
		if err != nil {
			t.Errorf("expected no error for valid rest days, got %v", err)
		}
	})
}
