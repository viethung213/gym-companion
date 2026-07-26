package domain

import (
	"errors"
	"fmt"
	"time"
)

var (
	ErrInvalidDailyPlanDate = errors.New("scheduled_date cannot be zero")
	ErrEmptyPrescription    = errors.New("workout prescription must contain at least 1 main exercise")
)

type DailyPlanStatus int32

const (
	DailyPlanStatusUnspecified DailyPlanStatus = 0
	DailyPlanStatusDraft       DailyPlanStatus = 1
	DailyPlanStatusActive      DailyPlanStatus = 2
	DailyPlanStatusCompleted   DailyPlanStatus = 3
	DailyPlanStatusSkipped     DailyPlanStatus = 4
)

type PrescribedExercise struct {
	exerciseID      string
	exerciseName    string
	targetSets      int32
	targetReps      int32
	targetWeight    float32
	durationSeconds int32
	notes           string
	restSetSec      int32
	restExerciseSec int32
	targetRPE       float32
}

func NewPrescribedExercise(id, name string, sets, reps int32, weight float32, durationSec int32, notes string, restSetSec, restExerciseSec int32, rpe float32) PrescribedExercise {
	return PrescribedExercise{
		exerciseID:      id,
		exerciseName:    name,
		targetSets:      sets,
		targetReps:      reps,
		targetWeight:    weight,
		durationSeconds: durationSec,
		notes:           notes,
		restSetSec:      restSetSec,
		restExerciseSec: restExerciseSec,
		targetRPE:       rpe,
	}
}

func (e PrescribedExercise) ExerciseID() string      { return e.exerciseID }
func (e PrescribedExercise) ExerciseName() string    { return e.exerciseName }
func (e PrescribedExercise) TargetSets() int32       { return e.targetSets }
func (e PrescribedExercise) TargetReps() int32       { return e.targetReps }
func (e PrescribedExercise) TargetWeight() float32   { return e.targetWeight }
func (e PrescribedExercise) DurationSeconds() int32 { return e.durationSeconds }
func (e PrescribedExercise) Notes() string           { return e.notes }
func (e PrescribedExercise) RestSetSec() int32      { return e.restSetSec }
func (e PrescribedExercise) RestExerciseSec() int32 { return e.restExerciseSec }
func (e PrescribedExercise) TargetRPE() float32       { return e.targetRPE }

type WorkoutPrescription struct {
	warmUps       []PrescribedExercise
	mainExercises []PrescribedExercise
	coolDowns     []PrescribedExercise
}

func NewWorkoutPrescription(warmUps, mainExercises, coolDowns []PrescribedExercise) WorkoutPrescription {
	return WorkoutPrescription{
		warmUps:       warmUps,
		mainExercises: mainExercises,
		coolDowns:     coolDowns,
	}
}

func (p WorkoutPrescription) WarmUps() []PrescribedExercise       { return p.warmUps }
func (p WorkoutPrescription) MainExercises() []PrescribedExercise { return p.mainExercises }
func (p WorkoutPrescription) CoolDowns() []PrescribedExercise     { return p.coolDowns }

// DailyWorkoutPlan represents a daily exercise prescription Aggregate Root.
type DailyWorkoutPlan struct {
	id                    string
	userID                string
	roadmapID             string
	weeklyScheduleID      string
	scheduledDate         time.Time
	status                DailyPlanStatus
	workoutPrescription   WorkoutPrescription
	reasoningExplanation  string
	adjustmentExplanation string
	generatedAt           time.Time
	createdAt             time.Time
	updatedAt             time.Time
}

func NewDailyWorkoutPlan(id, userID, roadmapID, weeklyScheduleID string, scheduledDate time.Time, status DailyPlanStatus, prescription WorkoutPrescription, reasoning, adjustment string) (*DailyWorkoutPlan, error) {
	if userID == "" || roadmapID == "" || weeklyScheduleID == "" {
		return nil, ErrInvalidUser
	}
	if scheduledDate.IsZero() {
		return nil, ErrInvalidDailyPlanDate
	}
	if len(prescription.MainExercises()) == 0 {
		return nil, ErrEmptyPrescription
	}
	if id == "" {
		id = fmt.Sprintf("dwp_%d", time.Now().UnixNano())
	}

	now := time.Now().UTC()
	return &DailyWorkoutPlan{
		id:                    id,
		userID:                userID,
		roadmapID:             roadmapID,
		weeklyScheduleID:      weeklyScheduleID,
		scheduledDate:         scheduledDate,
		status:                status,
		workoutPrescription:   prescription,
		reasoningExplanation:  reasoning,
		adjustmentExplanation: adjustment,
		generatedAt:           now,
		createdAt:             now,
		updatedAt:             now,
	}, nil
}

func ReconstituteDailyWorkoutPlan(id, userID, roadmapID, weeklyScheduleID string, scheduledDate time.Time, status DailyPlanStatus, prescription WorkoutPrescription, reasoning, adjustment string, generatedAt, createdAt, updatedAt time.Time) *DailyWorkoutPlan {
	return &DailyWorkoutPlan{
		id:                    id,
		userID:                userID,
		roadmapID:             roadmapID,
		weeklyScheduleID:      weeklyScheduleID,
		scheduledDate:         scheduledDate,
		status:                status,
		workoutPrescription:   prescription,
		reasoningExplanation:  reasoning,
		adjustmentExplanation: adjustment,
		generatedAt:           generatedAt,
		createdAt:             createdAt,
		updatedAt:             updatedAt,
	}
}

func (d *DailyWorkoutPlan) ID() string                    { return d.id }
func (d *DailyWorkoutPlan) UserID() string                { return d.userID }
func (d *DailyWorkoutPlan) RoadmapID() string             { return d.roadmapID }
func (d *DailyWorkoutPlan) WeeklyScheduleID() string      { return d.weeklyScheduleID }
func (d *DailyWorkoutPlan) ScheduledDate() time.Time      { return d.scheduledDate }
func (d *DailyWorkoutPlan) Status() DailyPlanStatus       { return d.status }
func (d *DailyWorkoutPlan) Prescription() WorkoutPrescription { return d.workoutPrescription }
func (d *DailyWorkoutPlan) ReasoningExplanation() string  { return d.reasoningExplanation }
func (d *DailyWorkoutPlan) AdjustmentExplanation() string { return d.adjustmentExplanation }
func (d *DailyWorkoutPlan) GeneratedAt() time.Time       { return d.generatedAt }
func (d *DailyWorkoutPlan) CreatedAt() time.Time         { return d.createdAt }
func (d *DailyWorkoutPlan) UpdatedAt() time.Time         { return d.updatedAt }

func (d *DailyWorkoutPlan) Activate() {
	d.status = DailyPlanStatusActive
	d.updatedAt = time.Now().UTC()
}

func (d *DailyWorkoutPlan) Complete() {
	d.status = DailyPlanStatusCompleted
	d.updatedAt = time.Now().UTC()
}
