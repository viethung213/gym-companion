package query

import (
	"context"
	"fmt"

	"github.com/viethung213/gym-companion/internal/coaching/application/port"
	"github.com/viethung213/gym-companion/internal/coaching/domain"
)

type Handler struct {
	repository port.Repository
}

func NewHandler(repository port.Repository) *Handler {
	return &Handler{repository: repository}
}

func (h *Handler) GetRoadmap(
	ctx context.Context,
	userID string,
	roadmapID string,
) (*domain.WorkoutRoadmap, error) {
	roadmap, err := h.repository.FindRoadmap(ctx, userID, roadmapID)
	if err != nil {
		return nil, fmt.Errorf("find roadmap: %w", err)
	}
	return roadmap, nil
}

func (h *Handler) ListRoadmaps(
	ctx context.Context,
	userID string,
) ([]*domain.WorkoutRoadmap, error) {
	roadmaps, err := h.repository.ListRoadmaps(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("list roadmaps: %w", err)
	}
	return roadmaps, nil
}

func (h *Handler) GetSchedule(
	ctx context.Context,
	userID string,
	scheduleID string,
) (*domain.WeeklySchedule, error) {
	schedule, err := h.repository.FindSchedule(ctx, userID, scheduleID)
	if err != nil {
		return nil, fmt.Errorf("find schedule: %w", err)
	}
	return schedule, nil
}

func (h *Handler) ListSchedules(
	ctx context.Context,
	userID string,
	roadmapID string,
) ([]*domain.WeeklySchedule, error) {
	schedules, err := h.repository.ListSchedules(ctx, userID, roadmapID)
	if err != nil {
		return nil, fmt.Errorf("list schedules: %w", err)
	}
	return schedules, nil
}
