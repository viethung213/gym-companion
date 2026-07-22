package domain

import "fmt"

type OverloadEngine struct{}

func NewOverloadEngine() *OverloadEngine {
	return &OverloadEngine{}
}

func (e *OverloadEngine) Calculate1RMEstimate(bodyWeight float64, exercise string) float64 {
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

func (e *OverloadEngine) EvaluateLog(actualReps, targetReps, rpe int, formScore float64) (ProgressionResult, error) {
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

func (e *OverloadEngine) NextWeight(currentWeight float64, prog ProgressionResult) float64 {
	switch prog {
	case ProgressionFastTrack:
		next := currentWeight * 1.05
		// Hard cap 10%
		if next > currentWeight*1.10 {
			next = currentWeight * 1.10
		}
		return next
	case ProgressionDownTrack:
		return currentWeight * 0.90
	default:
		return currentWeight
	}
}

func (e *OverloadEngine) CalculateWarmupSet(workingWeight float64, isCompound bool) *WorkoutPrescription {
	if !isCompound {
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
func (e *OverloadEngine) GetPeriodizationMultiplier(weekNumber int) float64 {
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

func (e *OverloadEngine) RestPeriod(category string, isBeginner bool) int {
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

