package domain

import (
	"fmt"

	"github.com/viethung213/gym-companion/internal/coaching/domain/services"
)

type OverloadEngine struct{}

func NewOverloadEngine() OverloadEngine {
	return OverloadEngine{}
}

func (e OverloadEngine) Calculate1RMEstimate(bodyWeight float64, exercise string) float64 {
	switch exercise {
	case "Bench Press":
		return bodyWeight * 0.50
	case "Squat":
		return bodyWeight * 0.70
	case "Deadlift":
		return bodyWeight * 0.90
	case "Overhead Press":
		return bodyWeight * 0.35
	case "Row":
		return bodyWeight * 0.60
	default:
		return 0
	}
}

type ProgressionResult string

const (
	ProgressionFastTrack ProgressionResult = "FAST_TRACK"
	ProgressionDownTrack ProgressionResult = "DOWN_TRACK"
	ProgressionMaintain  ProgressionResult = "MAINTAIN"
)

func (e OverloadEngine) EvaluateLog(actualReps, targetReps, rpe int, formScore float64) (ProgressionResult, error) {
	if actualReps > targetReps*2 {
		return ProgressionMaintain, fmt.Errorf("%w: suspicious log flagged", ErrOverloadHardCapExceeded)
	}

	if actualReps >= targetReps && rpe <= 7 && formScore >= 70 {
		return ProgressionFastTrack, nil
	}
	if actualReps < int(float64(targetReps)*0.8) || rpe >= 9 || formScore < 60 {
		return ProgressionDownTrack, nil
	}
	return ProgressionMaintain, nil
}

func (e OverloadEngine) NextWeight(currentWeight float64, prog ProgressionResult, isBeginner bool) float64 {
	switch prog {
	case ProgressionFastTrack:
		maxCapPct := 0.10
		if isBeginner {
			maxCapPct = 0.05 // BR-AC-12: max 5% overload per session for beginner
		}
		next := currentWeight * 1.05
		if next > currentWeight*(1+maxCapPct) {
			next = currentWeight * (1 + maxCapPct)
		}
		return next
	case ProgressionDownTrack:
		return currentWeight * 0.90
	default:
		return currentWeight
	}
}

func (e OverloadEngine) CalculateWarmupSet(workingWeight float64, isCompound bool) *WorkoutPrescription {
	if !isCompound || workingWeight <= 0 {
		return nil
	}
	wp, err := NewWorkoutPrescription(1, 10, workingWeight*0.50, 5.0, 90)
	if err != nil {
		return nil
	}
	return &wp
}

// GetPeriodizationMultiplier returns intensity multiplier (% 1RM) based on 4-week periodization scheme.
// Week 1: Adaptation (65% 1RM)
// Week 2: Build (75% 1RM)
// Week 3: Peak (85% 1RM)
// Week 4: Deload (55% 1RM)
func (e OverloadEngine) GetPeriodizationMultiplier(weekNumber int) float64 {
	switch weekNumber {
	case 1:
		return 0.65 // Adaptation
	case 2:
		return 0.75 // Build
	case 3:
		return 0.85 // Peak
	case 4:
		return 0.55 // Deload
	default:
		return 0.70
	}
}

func (e OverloadEngine) RestPeriod(category string, isBeginner bool) int {
	var base int
	switch category {
	case "Compound":
		base = 120
	case "Isolation":
		base = 60
	case "Cardio":
		base = 30
	default:
		base = 60
	}

	if isBeginner {
		base += 30
	}
	return base
}

type FitnessGoal string

const (
	FitnessGoalMuscleGain FitnessGoal = "MUSCLE_GAIN"
	FitnessGoalFatLoss    FitnessGoal = "FAT_LOSS"
)

func (g FitnessGoal) Valid() bool {
	switch g {
	case FitnessGoalMuscleGain, FitnessGoalFatLoss:
		return true
	default:
		return false
	}
}

// GeneratePrescription generates a WorkoutPrescription from Goal, PlanningTier, WeekNumber, 1RM, and Category.
func (e OverloadEngine) GeneratePrescription(
	goal FitnessGoal,
	planningTier string,
	weekNumber int,
	oneRM float64,
	category string,
) (WorkoutPrescription, error) {
	if !goal.Valid() {
		return WorkoutPrescription{}, fmt.Errorf("%w: invalid goal %s", ErrInvalidPrescription, goal)
	}

	var basePct float64
	var targetReps int
	var targetSets int

	switch goal {
	case FitnessGoalMuscleGain:
		basePct = 0.72
		targetReps = 10
		targetSets = 3
	case FitnessGoalFatLoss:
		basePct = 0.62
		targetReps = 12
		targetSets = 3
	}

	isBeginner := planningTier == "BEGINNER"
	if isBeginner {
		basePct -= 0.10 // 10% safety offset for beginner
		if targetSets > 3 {
			targetSets = 3
		}
	}

	periodizationMult := e.GetPeriodizationMultiplier(weekNumber)
	adjusted1RM := oneRM * basePct * periodizationMult
	workingWeight := services.CalculateSuggestedWeight(adjusted1RM, targetReps)

	restSeconds := e.RestPeriod(category, isBeginner)

	return NewWorkoutPrescription(targetSets, targetReps, workingWeight, 8.0, restSeconds)
}


// EstimateSessionDurationMinutes calculates estimated duration in minutes for a list of planned exercises.
func (e OverloadEngine) EstimateSessionDurationMinutes(exercises []PlannedExercise, isBeginner bool) int {
	totalSeconds := 0
	warmupBufferSeconds := 600 // 10 minutes warm-up buffer

	for _, pe := range exercises {
		sets := pe.Prescription().Sets()
		reps := pe.Prescription().Reps()
		rest := pe.Prescription().RestSeconds()

		exerciseTime := sets * (reps*3 + rest)
		totalSeconds += exerciseTime
	}

	totalSeconds += warmupBufferSeconds
	return (totalSeconds + 59) / 60
}



