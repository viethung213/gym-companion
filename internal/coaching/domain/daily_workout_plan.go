package domain

import (
	"fmt"
	"time"

	"github.com/google/uuid"
)

type PlannedExercise struct {
	id           string
	exerciseID   string
	prescription WorkoutPrescription
	notes        string
}

func NewPlannedExercise(exerciseID string, prescription WorkoutPrescription, notes string) (PlannedExercise, error) {
	if exerciseID == "" {
		return PlannedExercise{}, fmt.Errorf("%w: exerciseID cannot be empty", ErrInvalidPlan)
	}
	if err := prescription.Validate(); err != nil {
		return PlannedExercise{}, err
	}
	return PlannedExercise{
		id:           uuid.New().String(),
		exerciseID:   exerciseID,
		prescription: prescription,
		notes:        notes,
	}, nil
}

func (pe PlannedExercise) ID() string                     { return pe.id }
func (pe PlannedExercise) ExerciseID() string             { return pe.exerciseID }
func (pe PlannedExercise) Prescription() WorkoutPrescription { return pe.prescription }
func (pe PlannedExercise) Notes() string                    { return pe.notes }


type DailyWorkoutPlan struct {
	id         string
	scheduleID string
	userID     string
	status     PlanStatus
	exercises  []PlannedExercise
	createdAt  time.Time
	updatedAt  time.Time
}

func NewDailyWorkoutPlan(id, scheduleID, userID string, exercises []PlannedExercise, now time.Time) (*DailyWorkoutPlan, error) {
	if id == "" || scheduleID == "" || userID == "" {
		return nil, fmt.Errorf("%w: ids cannot be empty", ErrInvalidPlan)
	}
	if len(exercises) == 0 {
		return nil, fmt.Errorf("%w: exercises cannot be empty", ErrInvalidPlan)
	}

	exsCopy := make([]PlannedExercise, len(exercises))
	copy(exsCopy, exercises)

	return &DailyWorkoutPlan{
		id:         id,
		scheduleID: scheduleID,
		userID:     userID,
		status:     PlanStatusDraft,
		exercises:  exsCopy,
		createdAt:  now,
		updatedAt:  now,
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

func (d *DailyWorkoutPlan) AddExercise(pe PlannedExercise) error {
	if d.status == PlanStatusCompleted || d.status == PlanStatusCancelled {
		return fmt.Errorf("%w: cannot modify completed or cancelled plan", ErrInvalidPlan)
	}
	d.exercises = append(d.exercises, pe)
	return nil
}

func (d *DailyWorkoutPlan) ID() string         { return d.id }
func (d *DailyWorkoutPlan) ScheduleID() string { return d.scheduleID }
func (d *DailyWorkoutPlan) UserID() string     { return d.userID }
func (d *DailyWorkoutPlan) Status() PlanStatus { return d.status }

func (d *DailyWorkoutPlan) Exercises() []PlannedExercise {
	exs := make([]PlannedExercise, len(d.exercises))
	copy(exs, d.exercises)
	return exs
}

