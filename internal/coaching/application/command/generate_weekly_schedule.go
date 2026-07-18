package command

import (
	"context"
	"errors"
	"fmt"

	"github.com/viethung213/gym-companion/internal/coaching/application/port"
	"github.com/viethung213/gym-companion/internal/coaching/domain"
)

var ErrRoadmapCycleComplete = errors.New("roadmap already contains four weeks")

var (
	ErrPreviousScheduleRequired = errors.New("previous weekly schedule id is required")
	ErrPreviousScheduleMismatch = errors.New(
		"previous weekly schedule does not belong to the roadmap",
	)
)

type GenerateWeeklySchedule struct {
	UserID                   string
	RoadmapID                string
	PreviousWeeklyScheduleID string
}

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
	command GenerateWeeklySchedule,
) (*domain.WeeklySchedule, error) {
	if command.PreviousWeeklyScheduleID == "" {
		return nil, ErrPreviousScheduleRequired
	}
	roadmap, err := h.repository.FindRoadmap(ctx, command.UserID, command.RoadmapID)
	if err != nil {
		return nil, fmt.Errorf("find roadmap: %w", err)
	}
	previousSchedule, err := h.repository.FindSchedule(
		ctx,
		command.UserID,
		command.PreviousWeeklyScheduleID,
	)
	if err != nil {
		return nil, fmt.Errorf("find previous schedule: %w", err)
	}
	if previousSchedule.RoadmapID != roadmap.ID {
		return nil, ErrPreviousScheduleMismatch
	}
	nextWeek := previousSchedule.WeekNumber + 1
	if nextWeek > 4 {
		return nil, ErrRoadmapCycleComplete
	}
	if existing, findErr := h.repository.FindScheduleByWeek(
		ctx,
		roadmap.ID,
		nextWeek,
	); findErr == nil {
		return existing, nil
	} else if !errors.Is(findErr, domain.ErrNotFound) {
		return nil, fmt.Errorf("find schedule by week: %w", findErr)
	}

	scheduleID, err := h.ids.NewID()
	if err != nil {
		return nil, fmt.Errorf("generate schedule id: %w", err)
	}
	days, err := h.planner.PlanWeek(&roadmap.Input, nextWeek)
	if err != nil {
		return nil, fmt.Errorf("plan week: %w", err)
	}
	schedule, err := domain.NewWeeklySchedule(scheduleID, roadmap.ID, command.UserID, nextWeek, days)
	if err != nil {
		return nil, fmt.Errorf("create schedule: %w", err)
	}
	event := newEvent(
		scheduleID,
		command.UserID,
		"contracts.coaching.coachingService.v1.weeklyScheduleGenerated",
		h.clock.Now(),
	)
	if err := h.repository.SaveSchedule(ctx, schedule, &event); err != nil {
		return nil, fmt.Errorf("persist schedule: %w", err)
	}
	return schedule, nil
}
