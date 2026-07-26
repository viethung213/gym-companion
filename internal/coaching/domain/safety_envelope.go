package domain

import (
	"math"
	"strings"
)

// UpperSafetyEnvelopeValidator represents the Go Upper Safety Envelope Domain Service.
// It enforces BR-AC-01 (rest days), BR-AC-02 (+/- 30% Load Adjustment Ceiling), Deload RPE <= 6, and Active Injury Locks.
type UpperSafetyEnvelopeValidator struct{}

func NewUpperSafetyEnvelopeValidator() *UpperSafetyEnvelopeValidator {
	return &UpperSafetyEnvelopeValidator{}
}

// ValidateLoadCeiling enforces BR-AC-02: Load Adjustment Ceiling of +/- 30% relative to previous baseline weight.
// Returns (cappedWeight, wasAdjusted).
func (v *UpperSafetyEnvelopeValidator) ValidateLoadCeiling(prevBaselineWeight float32, proposedWeight float32) (float32, bool) {
	if prevBaselineWeight <= 0 {
		return proposedWeight, false
	}

	maxAllowed := prevBaselineWeight * 1.30
	minAllowed := prevBaselineWeight * 0.70

	if proposedWeight > maxAllowed {
		return maxAllowed, true
	}
	if proposedWeight < minAllowed {
		return minAllowed, true
	}

	return proposedWeight, false
}

// ValidateDeloadWeek enforces Decision 1.4: If Week 4, RPE must not exceed 6.0.
func (v *UpperSafetyEnvelopeValidator) ValidateDeloadWeek(weekNumber int32, exercises []PrescribedExercise) ([]PrescribedExercise, bool) {
	if weekNumber != 4 {
		return exercises, false
	}

	wasAdjusted := false
	result := make([]PrescribedExercise, len(exercises))
	for i, ex := range exercises {
		targetRPE := ex.targetRPE
		if targetRPE > 6.0 {
			targetRPE = 6.0
			wasAdjusted = true
		}
		result[i] = NewPrescribedExercise(
			ex.exerciseID,
			ex.exerciseName,
			ex.targetSets,
			ex.targetReps,
			ex.targetWeight,
			ex.durationSeconds,
			ex.notes,
			ex.restSetSec,
			ex.restExerciseSec,
			targetRPE,
		)
	}

	return result, wasAdjusted
}

// ValidateInjuryLocks filters out exercises that target active injury joints.
// Returns safe exercises and count of pruned exercises.
func (v *UpperSafetyEnvelopeValidator) ValidateInjuryLocks(exercises []PrescribedExercise, activeInjuryJoints []string) ([]PrescribedExercise, int) {
	if len(activeInjuryJoints) == 0 {
		return exercises, 0
	}

	injurySet := make(map[string]bool)
	for _, joint := range activeInjuryJoints {
		injurySet[strings.ToLower(strings.TrimSpace(joint))] = true
	}

	var safeExercises []PrescribedExercise
	prunedCount := 0

	for _, ex := range exercises {
		notesLower := strings.ToLower(ex.notes)
		isInjured := false

		for joint := range injurySet {
			if strings.Contains(notesLower, joint) {
				isInjured = true
				break
			}
		}

		if isInjured {
			prunedCount++
		} else {
			safeExercises = append(safeExercises, ex)
		}
	}

	return safeExercises, prunedCount
}

// ValidateRestDays enforces BR-AC-01 (at least 1 rest day, at most 6 training days).
func (v *UpperSafetyEnvelopeValidator) ValidateRestDays(schedule *WeeklySchedule) error {
	if schedule == nil {
		return ErrInvalidSchedule
	}
	trainingCount := 0
	restCount := 0
	for _, day := range schedule.ScheduleDays() {
		if day.IsTrainingDay() {
			trainingCount++
		} else if day.IsRestDay() {
			restCount++
		}
	}
	if restCount < 1 || trainingCount > 6 {
		return ErrViolationBRAC01
	}
	return nil
}

func roundTwoDecimals(val float32) float32 {
	return float32(math.Round(float64(val)*100) / 100)
}
