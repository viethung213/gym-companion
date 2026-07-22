package domain

import (
	"fmt"
	"time"
)

type PlanStatus string

const (
	PlanStatusDraft     PlanStatus = "DRAFT"
	PlanStatusActive    PlanStatus = "ACTIVE"
	PlanStatusCompleted PlanStatus = "COMPLETED"
	PlanStatusCancelled PlanStatus = "CANCELLED"
)

type PlanningTier string

const (
	PlanningTierBeginner    PlanningTier = "BEGINNER"
	PlanningTierExperienced PlanningTier = "EXPERIENCED"
)

type WorkoutRoadmap struct {
	id           string
	userID       string
	status       PlanStatus
	startDate    time.Time
	endDate      *time.Time
	planningTier PlanningTier
	createdAt    time.Time
	updatedAt    time.Time
}

func NewWorkoutRoadmap(id, userID string, tier PlanningTier, now time.Time) (*WorkoutRoadmap, error) {
	if id == "" || userID == "" {
		return nil, fmt.Errorf("%w: id and userID cannot be empty", ErrInvalidRoadmap)
	}
	return &WorkoutRoadmap{
		id:           id,
		userID:       userID,
		status:       PlanStatusDraft,
		startDate:    now,
		planningTier: tier,
		createdAt:    now,
		updatedAt:    now,
	}, nil
}

func (w *WorkoutRoadmap) Activate(now time.Time) error {
	if w.status != PlanStatusDraft {
		return fmt.Errorf("%w: can only activate draft roadmap", ErrInvalidStatusTransition)
	}
	w.status = PlanStatusActive
	w.updatedAt = now
	return nil
}

func (w *WorkoutRoadmap) Complete(now time.Time) error {
	if w.status != PlanStatusActive {
		return fmt.Errorf("%w: can only complete active roadmap", ErrInvalidStatusTransition)
	}
	w.status = PlanStatusCompleted
	w.endDate = &now
	w.updatedAt = now
	return nil
}

func (w *WorkoutRoadmap) ID() string { return w.id }
func (w *WorkoutRoadmap) UserID() string { return w.userID }
func (w *WorkoutRoadmap) Status() PlanStatus { return w.status }
func (w *WorkoutRoadmap) PlanningTier() PlanningTier { return w.planningTier }
