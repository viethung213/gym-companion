package domain

import (
	"fmt"
	"time"
)

type DailyWorkoutPlan struct {
	id           string
	scheduleID   string
	userID       string
	status       PlanStatus
	prescriptions []WorkoutPrescription
	createdAt    time.Time
	updatedAt    time.Time
}

func NewDailyWorkoutPlan(id, scheduleID, userID string, now time.Time) (*DailyWorkoutPlan, error) {
	if id == "" || scheduleID == "" || userID == "" {
		return nil, fmt.Errorf("%w: ids cannot be empty", ErrInvalidPlan)
	}
	return &DailyWorkoutPlan{
		id:           id,
		scheduleID:   scheduleID,
		userID:       userID,
		status:       PlanStatusDraft,
		createdAt:    now,
		updatedAt:    now,
	}, nil
}

func (d *DailyWorkoutPlan) Activate(now time.Time) error {
	if d.status != PlanStatusDraft {
		return fmt.Errorf("%w: can only activate draft plan", ErrInvalidStatusTransition)
	}
	d.status = PlanStatusActive
	d.updatedAt = now
	return nil
}

func (d *DailyWorkoutPlan) Complete(now time.Time) error {
	if d.status != PlanStatusActive {
		return fmt.Errorf("%w: can only complete active plan", ErrInvalidStatusTransition)
	}
	d.status = PlanStatusCompleted
	d.updatedAt = now
	return nil
}

func (d *DailyWorkoutPlan) Cancel(now time.Time) error {
	if d.status == PlanStatusCompleted {
		return fmt.Errorf("%w: cannot cancel completed plan", ErrInvalidStatusTransition)
	}
	d.status = PlanStatusCancelled
	d.updatedAt = now
	return nil
}

func (d *DailyWorkoutPlan) AddPrescription(p WorkoutPrescription) error {
	if d.status == PlanStatusCompleted || d.status == PlanStatusCancelled {
		return fmt.Errorf("%w: cannot modify completed or cancelled plan", ErrInvalidPlan)
	}
	d.prescriptions = append(d.prescriptions, p)
	return nil
}

func (d *DailyWorkoutPlan) Status() PlanStatus { return d.status }
