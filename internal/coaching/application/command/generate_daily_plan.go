package command

import (
	"context"
	"fmt"
	"time"

	"github.com/viethung213/gym-companion/internal/coaching/application/port"
	"github.com/viethung213/gym-companion/internal/coaching/domain"
)

type GenerateDailyPlanHandler struct {
	roadmapRepo      domain.WorkoutRoadmapRepository
	scheduleRepo     domain.WeeklyScheduleRepository
	dailyPlanRepo    domain.DailyWorkoutPlanRepository
	agent            port.CoachingAgent
	exerciseProvider port.ExerciseProvider
	publisher        port.OutboxPublisher
	validator        *domain.UpperSafetyEnvelopeValidator
}

func NewGenerateDailyPlanHandler(
	roadmapRepo domain.WorkoutRoadmapRepository,
	scheduleRepo domain.WeeklyScheduleRepository,
	dailyPlanRepo domain.DailyWorkoutPlanRepository,
	agent port.CoachingAgent,
	exerciseProvider port.ExerciseProvider,
	publisher port.OutboxPublisher,
	validator *domain.UpperSafetyEnvelopeValidator,
) *GenerateDailyPlanHandler {
	return &GenerateDailyPlanHandler{
		roadmapRepo:      roadmapRepo,
		scheduleRepo:     scheduleRepo,
		dailyPlanRepo:    dailyPlanRepo,
		agent:            agent,
		exerciseProvider: exerciseProvider,
		publisher:        publisher,
		validator:        validator,
	}
}

func (h *GenerateDailyPlanHandler) Execute(ctx context.Context, cmd port.GenerateDailyPlanCommand) (*domain.DailyWorkoutPlan, error) {
	if cmd.UserID == "" {
		return nil, domain.ErrInvalidUser
	}
	if cmd.ScheduledDate.IsZero() {
		return nil, domain.ErrInvalidDailyPlanDate
	}

	// 1. Fetch Active Roadmap & Current Weekly Schedule
	roadmap, err := h.roadmapRepo.FindActiveByUserID(ctx, cmd.UserID)
	if err != nil || roadmap == nil {
		return nil, fmt.Errorf("active roadmap not found for user: %w", err)
	}

	schedule, err := h.scheduleRepo.FindCurrentByRoadmapID(ctx, roadmap.ID(), 1)
	if err != nil || schedule == nil {
		return nil, fmt.Errorf("current weekly schedule not found: %w", err)
	}

	// Decision 1.1: Automatically mark any missed previous session as Skipped (BR-AC-03)
	schedule.MarkDaySkipped(cmd.ScheduledDate.AddDate(0, 0, -1))
	_ = h.scheduleRepo.Save(ctx, schedule)

	// Fetch Baseline 1RM
	baseline1RM, _ := h.exerciseProvider.GetBaseline1RM(ctx, cmd.UserID)

	// Decision 1.4: Check if Week 4 Deload
	isDeloadWeek := cmd.IsDeloadWeek || schedule.WeekNumber() == 4

	// 2. Execute Single-Agent ReAct Loop via Gemini Flash Agent
	agentOutput, err := h.agent.GenerateDailyPrescription(ctx, port.DailyPrescriptionParams{
		UserID:                  cmd.UserID,
		RoadmapID:               roadmap.ID(),
		WeeklyScheduleID:        schedule.ID(),
		ScheduledDate:           cmd.ScheduledDate,
		TargetMuscleGroups:      []string{"Chest", "Triceps"},
		AvailableEquipment:      []string{"Barbell", "Dumbbell"},
		ActiveInjuries:          nil,
		AnomalousSessionDetected: cmd.AnomalousSessionDetected,
		IsDeloadWeek:            isDeloadWeek,
		UserBaseline1RM:         baseline1RM,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to generate daily prescription: %w", err)
	}

	// 3. Post-AI Dual-Gate Upper Safety Envelope Validation
	prescription := agentOutput.Prescription

	// Decision 1.4: Cap RPE <= 6.0 if Deload Week
	cappedMainExs, _ := h.validator.ValidateDeloadWeek(schedule.WeekNumber(), prescription.MainExercises())

	// Decision 1.3: Enforce Load Adjustment Ceiling (+/- 30%) for each exercise
	finalMainExs := make([]domain.PrescribedExercise, len(cappedMainExs))
	for i, ex := range cappedMainExs {
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

	safePrescription := domain.NewWorkoutPrescription(prescription.WarmUps(), finalMainExs, prescription.CoolDowns())

	// 4. Create DailyWorkoutPlan Aggregate Root
	planID := fmt.Sprintf("dwp_%d", time.Now().UnixNano())
	dailyPlan, err := domain.NewDailyWorkoutPlan(
		planID,
		cmd.UserID,
		roadmap.ID(),
		schedule.ID(),
		cmd.ScheduledDate,
		domain.DailyPlanStatusActive,
		safePrescription,
		agentOutput.ReasoningExplanation,
		agentOutput.AdjustmentExplanation,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create daily workout plan aggregate: %w", err)
	}

	if err := h.dailyPlanRepo.Save(ctx, dailyPlan); err != nil {
		return nil, fmt.Errorf("failed to save daily workout plan: %w", err)
	}

	if h.publisher != nil {
		_ = h.publisher.PublishEvent(ctx, "DailyWorkoutPlanGenerated", dailyPlan.ID(), domain.DailyWorkoutPlanGeneratedEvent{
			DailyWorkoutPlanID: dailyPlan.ID(),
			WeeklyScheduleID:   schedule.ID(),
			RoadmapID:          roadmap.ID(),
			UserID:             dailyPlan.UserID(),
			ScheduledDate:      dailyPlan.ScheduledDate(),
			GeneratedAt:        time.Now().UTC(),
		})
	}

	return dailyPlan, nil
}
