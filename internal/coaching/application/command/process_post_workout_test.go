package command_test

import (
	"context"
	"testing"
	"time"

	"github.com/viethung213/gym-companion/internal/coaching/application/command"
	"github.com/viethung213/gym-companion/internal/coaching/application/port"
	"github.com/viethung213/gym-companion/internal/coaching/domain"
)

func TestProcessPostWorkoutHandler(t *testing.T) {
	t.Parallel()
	now := time.Now().UTC()
	dRepo := &mockDailyPlanRepo{}
	sRepo := &mockScheduleRepo{}
	agent := &mockCoachingAgent{}
	exProvider := &mockExerciseProvider{}
	validator := domain.NewUpperSafetyEnvelopeValidator()

	// Seed existing plan
	exs := []domain.PrescribedExercise{
		domain.NewPrescribedExercise("ex_1", "Bench", 3, 10, 60, 0, "", 90, 120, 7.5),
	}
	prescription := domain.NewWorkoutPrescription(nil, exs, nil)
	plan, _ := domain.NewDailyWorkoutPlan("dwp_1", "usr_100", "rdp_1", "ws_1", now, domain.DailyPlanStatusActive, prescription, "", "")
	dRepo.Save(context.Background(), plan)

	handler := command.NewProcessPostWorkoutHandler(dRepo, sRepo, agent, exProvider, validator)

	cmd := port.ProcessPostWorkoutCommand{
		UserID:          "usr_100",
		DailyPlanID:     "dwp_1",
		RPE:             7.5,
		FormScore:       88.5,
		CompletedSets:   3,
		CompletedReps:   30,
		MaxWeightLifted: 65.0,
		ActiveInjuries:  nil,
	}

	res, err := handler.Execute(context.Background(), cmd)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	if res.NextDailyPlanDraft == nil {
		t.Fatalf("expected pre-cached next daily plan draft")
	}

	if res.NextDailyPlanDraft.Status() != domain.DailyPlanStatusDraft {
		t.Errorf("expected draft status, got %v", res.NextDailyPlanDraft.Status())
	}
}
