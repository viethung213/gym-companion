package domain

import (
	"errors"
	"fmt"
	"time"
)

// DailyPlanStatus represents the lifecycle state of a DailyWorkoutPlan.
type DailyPlanStatus string

const (
	DailyPlanStatusDraft     DailyPlanStatus = "DRAFT"
	DailyPlanStatusActive    DailyPlanStatus = "ACTIVE"
	DailyPlanStatusCompleted DailyPlanStatus = "COMPLETED"
	DailyPlanStatusSkipped   DailyPlanStatus = "SKIPPED"
)

var (
	ErrInvalidPlan           = errors.New("invalid daily workout plan")
	ErrPlanInvalidStatus     = errors.New("invalid daily plan status")
	ErrPlanInvalidTransition = errors.New("invalid daily plan status transition")
)

// PrescribedExercise is a Value Object representing one exercise prescription within a plan.
type PrescribedExercise struct {
	ExerciseID      string
	ExerciseName    string
	TargetSets      int
	TargetReps      int
	TargetWeight    float64
	DurationSeconds int
	Notes           string
}

// WorkoutPrescription is a Value Object containing the full exercise prescription for one day.
type WorkoutPrescription struct {
	WarmUps       []PrescribedExercise
	MainExercises []PrescribedExercise
	CoolDowns     []PrescribedExercise
}

// DailyWorkoutPlan is the Aggregate Root for a specific day's workout prescription,
// generated JIT to avoid locking WeeklySchedule.
type DailyWorkoutPlan struct {
	id                      string
	weeklyScheduleID        string
	roadmapID               string
	userID                  string
	scheduledDate           time.Time
	status                  DailyPlanStatus
	workoutPrescription     WorkoutPrescription
	reasoningExplanation    string
	adjustmentExplanation   string
	createdAt               time.Time
	updatedAt               time.Time
}

// CreateDailyPlan creates a new DailyWorkoutPlan in DRAFT status.
func CreateDailyPlan(
	id, weeklyScheduleID, roadmapID, userID string,
	scheduledDate time.Time,
	prescription WorkoutPrescription,
	reasoning string,
	now time.Time,
) (*DailyWorkoutPlan, error) {
	if id == "" {
		return nil, fmt.Errorf("%w: id is required", ErrInvalidPlan)
	}
	if weeklyScheduleID == "" {
		return nil, fmt.Errorf("%w: weekly_schedule_id is required", ErrInvalidPlan)
	}
	if roadmapID == "" {
		return nil, fmt.Errorf("%w: roadmap_id is required", ErrInvalidPlan)
	}
	if userID == "" {
		return nil, fmt.Errorf("%w: user_id is required", ErrInvalidPlan)
	}

	return &DailyWorkoutPlan{
		id:                   id,
		weeklyScheduleID:     weeklyScheduleID,
		roadmapID:            roadmapID,
		userID:               userID,
		scheduledDate:        scheduledDate,
		status:               DailyPlanStatusDraft,
		workoutPrescription:  prescription,
		reasoningExplanation: reasoning,
		createdAt:            now,
		updatedAt:            now,
	}, nil
}

// RehydrateDailyPlan reconstructs a DailyWorkoutPlan from persistence.
func RehydrateDailyPlan(
	id, weeklyScheduleID, roadmapID, userID string,
	scheduledDate time.Time,
	status DailyPlanStatus,
	prescription WorkoutPrescription,
	reasoning, adjustment string,
	createdAt, updatedAt time.Time,
) (*DailyWorkoutPlan, error) {
	if !status.Valid() {
		return nil, fmt.Errorf("%w: %s", ErrPlanInvalidStatus, status)
	}
	return &DailyWorkoutPlan{
		id:                    id,
		weeklyScheduleID:      weeklyScheduleID,
		roadmapID:             roadmapID,
		userID:                userID,
		scheduledDate:         scheduledDate,
		status:                status,
		workoutPrescription:   prescription,
		reasoningExplanation:  reasoning,
		adjustmentExplanation: adjustment,
		createdAt:             createdAt,
		updatedAt:             updatedAt,
	}, nil
}

func (p *DailyWorkoutPlan) ID() string                          { return p.id }
func (p *DailyWorkoutPlan) WeeklyScheduleID() string            { return p.weeklyScheduleID }
func (p *DailyWorkoutPlan) RoadmapID() string                   { return p.roadmapID }
func (p *DailyWorkoutPlan) UserID() string                      { return p.userID }
func (p *DailyWorkoutPlan) ScheduledDate() time.Time            { return p.scheduledDate }
func (p *DailyWorkoutPlan) Status() DailyPlanStatus             { return p.status }
func (p *DailyWorkoutPlan) WorkoutPrescription() WorkoutPrescription { return p.workoutPrescription }
func (p *DailyWorkoutPlan) ReasoningExplanation() string        { return p.reasoningExplanation }
func (p *DailyWorkoutPlan) AdjustmentExplanation() string       { return p.adjustmentExplanation }
func (p *DailyWorkoutPlan) CreatedAt() time.Time                { return p.createdAt }
func (p *DailyWorkoutPlan) UpdatedAt() time.Time                { return p.updatedAt }

// Activate transitions the plan from DRAFT to ACTIVE.
func (p *DailyWorkoutPlan) Activate(now time.Time) error {
	if p.status != DailyPlanStatusDraft {
		return fmt.Errorf("%w: %s to %s", ErrPlanInvalidTransition, p.status, DailyPlanStatusActive)
	}
	p.status = DailyPlanStatusActive
	p.updatedAt = now
	return nil
}

// Complete transitions the plan to COMPLETED.
func (p *DailyWorkoutPlan) Complete(now time.Time) error {
	if p.status != DailyPlanStatusActive {
		return fmt.Errorf("%w: %s to %s", ErrPlanInvalidTransition, p.status, DailyPlanStatusCompleted)
	}
	p.status = DailyPlanStatusCompleted
	p.updatedAt = now
	return nil
}

// Skip marks the plan as SKIPPED.
func (p *DailyWorkoutPlan) Skip(now time.Time) error {
	if p.status == DailyPlanStatusCompleted || p.status == DailyPlanStatusSkipped {
		return fmt.Errorf("%w: %s to %s", ErrPlanInvalidTransition, p.status, DailyPlanStatusSkipped)
	}
	p.status = DailyPlanStatusSkipped
	p.updatedAt = now
	return nil
}

func (s DailyPlanStatus) Valid() bool {
	switch s {
	case DailyPlanStatusDraft, DailyPlanStatusActive, DailyPlanStatusCompleted, DailyPlanStatusSkipped:
		return true
	default:
		return false
	}
}
