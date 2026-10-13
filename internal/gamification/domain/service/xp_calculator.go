package service

import (
	"errors"
	"math"
)

var (
	// ErrInsufficientWorkout indicates a workout session that failed Proof of Workout criteria.
	ErrInsufficientWorkout = errors.New("insufficient workout session: minimum 15 minutes and 3 sets required")
)

const (
	// BaseWorkoutXp is the flat reward for completing any valid workout.
	BaseWorkoutXp = int32(50)

	// NutritionDailyXpReward is the fixed XP reward for meeting daily nutrition targets.
	NutritionDailyXpReward = int32(30)

	// MinimumSets required for Proof of Workout.
	MinimumSets = int32(3)
)

// WorkoutXpInput contains all metrics needed to calculate workout XP.
type WorkoutXpInput struct {
	TotalSets        int32
	TotalVolumeKg    float64
	VolumeRatio      *float64 // Optional planned vs actual volume ratio
	AverageFormScore *float32 // 0.0..1.0 or 0..100 scale
	HasNewPR         bool
	CurrentStreak    int32
}

// CalculateWorkoutXp computes the total XP earned from a completed workout session according to BR-XP-02 and ADR-0002.
// Trust Model (ADR-0002): Gamification relies on workout_execution boundary for workout session validity.
// Formula: Total = round((Base + Volume + Form + PR) * StreakMultiplier)
func CalculateWorkoutXp(input WorkoutXpInput) (int32, error) {
	if input.TotalSets < MinimumSets {
		return 0, ErrInsufficientWorkout
	}

	base := BaseWorkoutXp

	// 1. Volume Component (clamp between 10 and 30)
	volume := int32(10)
	if input.VolumeRatio != nil {
		ratio := *input.VolumeRatio
		scaled := int32(math.Round(25.0 * ratio))
		if scaled < 10 {
			volume = 10
		} else if scaled > 30 {
			volume = 30
		} else {
			volume = scaled
		}
	} else {
		if input.TotalVolumeKg >= 5000.0 {
			volume = 30
		} else if input.TotalVolumeKg >= 1000.0 {
			volume = 20
		}
	}

	// 2. Form Score Component: round(20 * (FormScore / 100))
	form := int32(0)
	if input.AverageFormScore != nil {
		score := float64(*input.AverageFormScore)
		if score > 1.0 {
			score = score / 100.0 // Normalize 0..100 to 0.0..1.0
		}
		if score < 0.0 {
			score = 0.0
		} else if score > 1.0 {
			score = 1.0
		}
		form = int32(math.Round(20.0 * score))
	}

	// 3. PR Component: +25 XP if new PR
	pr := int32(0)
	if input.HasNewPR {
		pr = 25
	}

	rawTotal := base + volume + form + pr

	// 4. Streak Multiplier (BR-XP-02: streak >= 7 -> 1.20, streak >= 3 -> 1.10, default -> 1.00)
	multiplier := 1.00
	if input.CurrentStreak >= 7 {
		multiplier = 1.20
	} else if input.CurrentStreak >= 3 {
		multiplier = 1.10
	}

	finalXp := int32(math.Round(float64(rawTotal) * multiplier))
	return finalXp, nil
}
