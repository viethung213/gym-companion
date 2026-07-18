package command

import (
	"context"
	"errors"
	"fmt"

	"github.com/viethung213/gym-companion/internal/coaching/application/port"
	"github.com/viethung213/gym-companion/internal/coaching/domain"
)

var ErrRoadmapCycleComplete = errors.New("roadmap already contains four weeks")

type GenerateWeeklyScheduleHandler struct {
	repository port.Repository
	clock      port.Clock
	ids        port.IDGenerator
	planner    domain.SchedulePlanner
}

func NewGenerateWeeklyScheduleHandler(
	repository port.Repository,
	clock port.Clock,
	ids port.IDGenerator,
) *GenerateWeeklyScheduleHandler {
	return &GenerateWeeklyScheduleHandler{repository: repository, clock: clock, ids: ids}
}

func (h *GenerateWeeklyScheduleHandler) Handle(
	ctx context.Context,
	userID string,
	roadmapID string,
) (*domain.WeeklySchedule, error) {
	roadmap, err := h.repository.FindRoadmap(ctx, userID, roadmapID)
	if err != nil {
		return nil, fmt.Errorf("find roadmap: %w", err)
	}
	schedules, err := h.repository.ListSchedules(ctx, userID, roadmapID)
	if err != nil {
		return nil, fmt.Errorf("list schedules: %w", err)
	}
	nextWeek := len(schedules) + 1
	if nextWeek > 4 {
		return nil, ErrRoadmapCycleComplete
	}
	if existing, findErr := h.repository.FindScheduleByWeek(ctx, roadmapID, nextWeek); findErr == nil {
		return existing, nil
	} else if !errors.Is(findErr, domain.ErrNotFound) {
		return nil, fmt.Errorf("find schedule by week: %w", findErr)
	}

	scheduleID, err := h.ids.NewID()
	if err != nil {
		return nil, fmt.Errorf("generate schedule id: %w", err)
	}
	days, err := h.planner.PlanWeek(roadmap.Input, nextWeek)
	if err != nil {
		return nil, fmt.Errorf("plan week: %w", err)
	}
	schedule, err := domain.NewWeeklySchedule(scheduleID, roadmapID, userID, nextWeek, days)
	if err != nil {
		return nil, fmt.Errorf("create schedule: %w", err)
	}
	event := newEvent(
		scheduleID,
		userID,
		"contracts.coaching.coachingService.v1.weeklyScheduleGenerated",
		h.clock.Now(),
	)
	if err := h.repository.SaveSchedule(ctx, schedule, event); err != nil {
		return nil, fmt.Errorf("persist schedule: %w", err)
	}
	return schedule, nil
}
