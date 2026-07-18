package command

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/viethung213/gym-companion/internal/coaching/application/port"
	"github.com/viethung213/gym-companion/internal/coaching/domain"
)

var ErrActiveRoadmapExists = errors.New("an active roadmap already exists")

type InitiateRoadmap struct {
	UserID         string
	PlanningInput  domain.PlanningInput
	PlannerVersion string
}

type InitiateRoadmapResult struct {
	Roadmap  *domain.WorkoutRoadmap
	Schedule *domain.WeeklySchedule
}

type InitiateRoadmapHandler struct {
	repository port.Repository
	clock      port.Clock
	ids        port.IDGenerator
	planner    domain.SchedulePlanner
}

func NewInitiateRoadmapHandler(
	repository port.Repository,
	clock port.Clock,
	ids port.IDGenerator,
) *InitiateRoadmapHandler {
	return &InitiateRoadmapHandler{repository: repository, clock: clock, ids: ids}
}

func (h *InitiateRoadmapHandler) Handle(
	ctx context.Context,
	command InitiateRoadmap,
) (*InitiateRoadmapResult, error) {
	active, err := h.repository.FindActiveRoadmapByUser(ctx, command.UserID)
	if err != nil && !errors.Is(err, domain.ErrNotFound) {
		return nil, fmt.Errorf("find active roadmap: %w", err)
	}
	if active != nil {
		return nil, ErrActiveRoadmapExists
	}

	roadmapID, err := h.ids.NewID()
	if err != nil {
		return nil, fmt.Errorf("generate roadmap id: %w", err)
	}
	scheduleID, err := h.ids.NewID()
	if err != nil {
		return nil, fmt.Errorf("generate schedule id: %w", err)
	}
	roadmap, err := domain.NewWorkoutRoadmap(
		roadmapID,
		command.UserID,
		command.PlanningInput,
		command.PlannerVersion,
	)
	if err != nil {
		return nil, fmt.Errorf("create roadmap: %w", err)
	}
	days, err := h.planner.PlanWeek(command.PlanningInput, 1)
	if err != nil {
		return nil, fmt.Errorf("plan first week: %w", err)
	}
	schedule, err := domain.NewWeeklySchedule(scheduleID, roadmapID, command.UserID, 1, days)
	if err != nil {
		return nil, fmt.Errorf("create first schedule: %w", err)
	}

	now := h.clock.Now()
	events := []domain.Event{
		newEvent(roadmapID, command.UserID, "contracts.coaching.coachingService.v1.roadmapInitiated", now),
		newEvent(scheduleID, command.UserID, "contracts.coaching.coachingService.v1.weeklyScheduleGenerated", now),
	}
	if err := h.repository.CreateRoadmapWithSchedule(ctx, roadmap, schedule, events); err != nil {
		return nil, fmt.Errorf("persist roadmap and first schedule: %w", err)
	}

	return &InitiateRoadmapResult{Roadmap: roadmap, Schedule: schedule}, nil
}

func newEvent(subject string, userID string, eventType string, eventTime time.Time) domain.Event {
	return domain.Event{
		ID:           subject,
		Type:         eventType,
		Source:       "/coaching",
		Subject:      subject,
		PartitionKey: userID,
		Time:         eventTime,
		Data:         map[string]any{"userId": userID},
	}
}
