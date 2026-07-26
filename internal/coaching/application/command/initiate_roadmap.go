package command

import (
	"context"
	"fmt"
	"time"

	"github.com/viethung213/gym-companion/internal/coaching/application/port"
	"github.com/viethung213/gym-companion/internal/coaching/domain"
)

type InitiateRoadmapHandler struct {
	roadmapRepo    domain.WorkoutRoadmapRepository
	scheduleRepo   domain.WeeklyScheduleRepository
	agent          port.CoachingAgent
	publisher      port.OutboxPublisher
	validator      *domain.UpperSafetyEnvelopeValidator
}

func NewInitiateRoadmapHandler(
	roadmapRepo domain.WorkoutRoadmapRepository,
	scheduleRepo domain.WeeklyScheduleRepository,
	agent port.CoachingAgent,
	publisher port.OutboxPublisher,
	validator *domain.UpperSafetyEnvelopeValidator,
) *InitiateRoadmapHandler {
	return &InitiateRoadmapHandler{
		roadmapRepo:  roadmapRepo,
		scheduleRepo: scheduleRepo,
		agent:        agent,
		publisher:    publisher,
		validator:    validator,
	}
}

func (h *InitiateRoadmapHandler) Execute(ctx context.Context, cmd port.InitiateRoadmapCommand) (*port.InitiateRoadmapResult, error) {
	if cmd.UserID == "" {
		return nil, domain.ErrInvalidUser
	}

	// 1. Generate Roadmap Strategy via Gemini Pro Agent (Step 1)
	stratOutput, err := h.agent.GenerateRoadmapStrategy(ctx, port.RoadmapStrategyParams{
		UserID:             cmd.UserID,
		Goal:               cmd.Goal,
		ExperienceLevel:    cmd.ExperienceLevel,
		AvailableSlots:     cmd.AvailableSlots,
		AvailableEquipment: cmd.AvailableEquipment,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to generate roadmap strategy: %w", err)
	}

	roadmapID := fmt.Sprintf("rdp_%d", time.Now().UnixNano())
	roadmap, err := domain.NewWorkoutRoadmap(roadmapID, cmd.UserID, stratOutput.StartDate, stratOutput.EndDate)
	if err != nil {
		return nil, fmt.Errorf("failed to create roadmap aggregate: %w", err)
	}

	if err := h.roadmapRepo.Save(ctx, roadmap); err != nil {
		return nil, fmt.Errorf("failed to save roadmap: %w", err)
	}

	if h.publisher != nil {
		_ = h.publisher.PublishEvent(ctx, "RoadmapInitiated", roadmap.ID(), domain.RoadmapInitiatedEvent{
			RoadmapID:   roadmap.ID(),
			UserID:      roadmap.UserID(),
			StartDate:   roadmap.StartDate(),
			EndDate:     roadmap.EndDate(),
			InitiatedAt: time.Now().UTC(),
		})
	}

	// 2. Generate Week 1 Schedule via Gemini Flash Agent (Step 2)
	schedOutput, err := h.agent.GenerateWeeklySchedule(ctx, port.WeeklyScheduleParams{
		UserID:         cmd.UserID,
		RoadmapID:      roadmap.ID(),
		WeekNumber:     1,
		Goal:           cmd.Goal,
		AvailableSlots: cmd.AvailableSlots,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to generate weekly schedule: %w", err)
	}

	scheduleID := fmt.Sprintf("ws_%d", time.Now().UnixNano())
	schedule, err := domain.NewWeeklySchedule(
		scheduleID,
		roadmap.ID(),
		cmd.UserID,
		1,
		stratOutput.StartDate,
		stratOutput.StartDate.AddDate(0, 0, 7),
		schedOutput.MuscleSplitType,
		schedOutput.Days,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create weekly schedule aggregate: %w", err)
	}

	if err := h.validator.ValidateRestDays(schedule); err != nil {
		return nil, fmt.Errorf("weekly schedule safety validation failed: %w", err)
	}

	if err := h.scheduleRepo.Save(ctx, schedule); err != nil {
		return nil, fmt.Errorf("failed to save weekly schedule: %w", err)
	}

	if h.publisher != nil {
		_ = h.publisher.PublishEvent(ctx, "WeeklyScheduleGenerated", schedule.ID(), domain.WeeklyScheduleGeneratedEvent{
			WeeklyScheduleID: schedule.ID(),
			RoadmapID:        roadmap.ID(),
			UserID:           schedule.UserID(),
			WeekNumber:       1,
			StartDate:        schedule.StartDate(),
			EndDate:          schedule.EndDate(),
			GeneratedAt:      time.Now().UTC(),
		})
	}

	return &port.InitiateRoadmapResult{
		Roadmap:        roadmap,
		WeeklySchedule: schedule,
	}, nil
}
