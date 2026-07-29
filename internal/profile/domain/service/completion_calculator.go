package service

import (
	"github.com/viethung213/gym-companion/internal/profile/domain/entity"
	"github.com/viethung213/gym-companion/internal/profile/domain/vo"
)

type CompletionResult struct {
	CompletionRate   float64
	AICoachActivated bool
}

type CompletionCalculator struct{}

func NewCompletionCalculator() *CompletionCalculator {
	return &CompletionCalculator{}
}

func (c *CompletionCalculator) Calculate(
	bio vo.BiologicalMetrics,
	experienceLevel string,
	goals []string,
	preferredWorkoutTimes []string,
	availableEquipment []string,
	preferredMuscleGroups []string,
	coachStyle string,
	targetWeightKg float64,
	injuries []*entity.Injury,
) CompletionResult {
	_ = targetWeightKg
	_ = injuries

	totalPoints := 10.0
	var earnedPoints float64

	if bio.WeightKg() > 0 {
		earnedPoints += 1.0
	}
	if bio.HeightCm() > 0 {
		earnedPoints += 1.0
	}
	if bio.Age() > 0 {
		earnedPoints += 1.0
	}
	if bio.Gender() != "" {
		earnedPoints += 1.0
	}
	if experienceLevel != "" {
		earnedPoints += 1.0
	}
	if len(goals) > 0 {
		earnedPoints += 1.0
	}
	if len(preferredWorkoutTimes) > 0 {
		earnedPoints += 1.0
	}
	if len(availableEquipment) > 0 {
		earnedPoints += 1.0
	}
	if len(preferredMuscleGroups) > 0 {
		earnedPoints += 1.0
	}
	if coachStyle != "" {
		earnedPoints += 1.0
	}

	rate := (earnedPoints / totalPoints) * 100.0
	activated := rate >= 80.0

	return CompletionResult{
		CompletionRate:   rate,
		AICoachActivated: activated,
	}
}
