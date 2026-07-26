package command

import (
	"context"
	"fmt"
	"time"

	"github.com/viethung213/gym-companion/internal/coaching/application/port"
	"github.com/viethung213/gym-companion/internal/coaching/domain"
)

type ProcessPostWorkoutHandler struct {
	dailyPlanRepo    domain.DailyWorkoutPlanRepository
	scheduleRepo     domain.WeeklyScheduleRepository
	agent            port.CoachingAgent
	exerciseProvider port.ExerciseProvider
	validator        *domain.UpperSafetyEnvelopeValidator
}

func NewProcessPostWorkoutHandler(
	dailyPlanRepo domain.DailyWorkoutPlanRepository,
	scheduleRepo domain.WeeklyScheduleRepository,
	agent port.CoachingAgent,
	exerciseProvider port.ExerciseProvider,
	validator *domain.UpperSafetyEnvelopeValidator,
) *ProcessPostWorkoutHandler {
	return &ProcessPostWorkoutHandler{
		dailyPlanRepo:    dailyPlanRepo,
		scheduleRepo:     scheduleRepo,
		agent:            agent,
		exerciseProvider: exerciseProvider,
		validator:        validator,
	}
}

func (h *ProcessPostWorkoutHandler) Execute(ctx context.Context, cmd port.ProcessPostWorkoutCommand) (*port.ProcessPostWorkoutResult, error) {
	if cmd.UserID == "" || cmd.DailyPlanID == "" {
		return nil, domain.ErrInvalidUser
	}

	// 1. Fetch current daily plan & mark completed
	currentPlan, err := h.dailyPlanRepo.FindByID(ctx, cmd.DailyPlanID)
	if err != nil || currentPlan == nil {
		return nil, fmt.Errorf("daily plan not found: %w", err)
	}

	currentPlan.Complete()
	_ = h.dailyPlanRepo.Save(ctx, currentPlan)

	// 2. Pre-cache Next Session Daily Workout Plan (Session N+1)
	nextDate := currentPlan.ScheduledDate().AddDate(0, 0, 1)

	// Fetch 1RM baseline
	baseline1RM, _ := h.exerciseProvider.GetBaseline1RM(ctx, cmd.UserID)

	// Generate Draft Prescription for Session N+1
	agentOutput, err := h.agent.GenerateDailyPrescription(ctx, port.DailyPrescriptionParams{
		UserID:                  cmd.UserID,
		RoadmapID:               currentPlan.RoadmapID(),
		WeeklyScheduleID:        currentPlan.WeeklyScheduleID(),
		ScheduledDate:           nextDate,
		TargetMuscleGroups:      []string{"Back", "Biceps"},
		AvailableEquipment:      []string{"Barbell", "Dumbbell"},
		ActiveInjuries:          cmd.ActiveInjuries,
		AnomalousSessionDetected: false,
		IsDeloadWeek:            false,
		UserBaseline1RM:         baseline1RM,
	})
	if err != nil {
		return &port.ProcessPostWorkoutResult{
			CoachMessage:       "Great job completing your workout!",
			NextDailyPlanDraft: nil,
		}, nil
	}

	// Apply Safety Envelope (+/- 30% Load Adjustment Ceiling)
	finalMainExs := make([]domain.PrescribedExercise, len(agentOutput.Prescription.MainExercises()))
	for i, ex := range agentOutput.Prescription.MainExercises() {
		prevBaseline := baseline1RM[ex.ExerciseID()]
		cappedWeight, _ := h.validator.ValidateLoadCeiling(prevBaseline, ex.TargetWeight())

		finalMainExs[i] = domain.NewPrescribedExercise(
			ex.ExerciseID(),
			ex.ExerciseName(),
			ex.TargetSets(),
			ex.TargetReps(),
			cappedWeight,
			ex.DurationSeconds(),
			ex.Notes(),
			ex.RestSetSec(),
			ex.RestExerciseSec(),
			ex.TargetRPE(),
		)
	}

	draftPrescription := domain.NewWorkoutPrescription(
		agentOutput.Prescription.WarmUps(),
		finalMainExs,
		agentOutput.Prescription.CoolDowns(),
	)

	draftPlanID := fmt.Sprintf("dwp_draft_%d", time.Now().UnixNano())
	draftPlan, err := domain.NewDailyWorkoutPlan(
		draftPlanID,
		cmd.UserID,
		currentPlan.RoadmapID(),
		currentPlan.WeeklyScheduleID(),
		nextDate,
		domain.DailyPlanStatusDraft, // Pre-cached Draft
		draftPrescription,
		agentOutput.ReasoningExplanation,
		agentOutput.AdjustmentExplanation,
	)
	if err == nil {
		_ = h.dailyPlanRepo.Save(ctx, draftPlan)
	}

	coachMsg := fmt.Sprintf("Awesome effort today! Form Score: %.1f%%. Your next session is pre-cached and ready.", cmd.FormScore)

	return &port.ProcessPostWorkoutResult{
		CoachMessage:       coachMsg,
		NextDailyPlanDraft: draftPlan,
	}, nil
}
