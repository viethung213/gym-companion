package ai

import (
	"context"
	"time"

	"github.com/viethung213/gym-companion/internal/coaching/application/port"
	"github.com/viethung213/gym-companion/internal/coaching/domain"
)

type GeminiCoachAgent struct {
	apiKey string
}

func NewGeminiCoachAgent(apiKey string) *GeminiCoachAgent {
	return &GeminiCoachAgent{apiKey: apiKey}
}

func (a *GeminiCoachAgent) GenerateRoadmapStrategy(ctx context.Context, params port.RoadmapStrategyParams) (*port.RoadmapStrategyOutput, error) {
	now := time.Now().UTC()
	return &port.RoadmapStrategyOutput{
		StartDate: now,
		EndDate:   now.AddDate(0, 0, 28),
	}, nil
}

func (a *GeminiCoachAgent) GenerateWeeklySchedule(ctx context.Context, params port.WeeklyScheduleParams) (*port.WeeklyScheduleOutput, error) {
	now := time.Now().UTC()
	days := make([]domain.ScheduleDay, 7)
	daysOfWeek := []string{"Monday", "Tuesday", "Wednesday", "Thursday", "Friday", "Saturday", "Sunday"}

	for i := 0; i < 7; i++ {
		st := domain.WorkoutDayStatusTraining
		if i == 6 {
			st = domain.WorkoutDayStatusRest // 1 Rest day on Sunday (BR-AC-01)
		}
		targetMuscles := []string{"Push"}
		if i%2 == 1 {
			targetMuscles = []string{"Pull"}
		}
		days[i] = domain.NewScheduleDay(now.AddDate(0, 0, i), daysOfWeek[i], st, targetMuscles, "")
	}

	return &port.WeeklyScheduleOutput{
		MuscleSplitType: "Push/Pull/Legs",
		Days:            days,
	}, nil
}

func (a *GeminiCoachAgent) GenerateDailyPrescription(ctx context.Context, params port.DailyPrescriptionParams) (*port.DailyPrescriptionOutput, error) {
	targetRPE := float32(7.5)
	if params.IsDeloadWeek {
		targetRPE = 5.5 // Deload RPE <= 6.0
	}

	note := "Standard execution"
	if params.AnomalousSessionDetected {
		note = "Active Recovery session forced due to previous anomalous session"
	}

	mainExs := []domain.PrescribedExercise{
		domain.NewPrescribedExercise("ex_bench", "Barbell Bench Press", 3, 10, 60.0, 0, note, 90, 120, targetRPE),
		domain.NewPrescribedExercise("ex_incline", "Incline Dumbbell Press", 3, 12, 22.0, 0, note, 90, 120, targetRPE),
	}

	warmUps := []domain.PrescribedExercise{
		domain.NewPrescribedExercise("ex_arm_circles", "Arm Circles", 2, 15, 0, 30, "Warm-up shoulder joint", 30, 45, 4.0),
	}

	coolDowns := []domain.PrescribedExercise{
		domain.NewPrescribedExercise("ex_chest_stretch", "Doorway Chest Stretch", 2, 1, 0, 45, "Hold static stretch", 30, 30, 3.0),
	}

	return &port.DailyPrescriptionOutput{
		Prescription:         domain.NewWorkoutPrescription(warmUps, mainExs, coolDowns),
		ReasoningExplanation: "Selected compound movements targeting chest and triceps with progressive overload.",
		AdjustmentExplanation: "",
	}, nil
}
