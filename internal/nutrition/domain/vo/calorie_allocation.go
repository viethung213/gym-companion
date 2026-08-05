package vo

import (
	"errors"
	"fmt"
)

var ErrInvalidCalories = errors.New("target calories must be at least 1200 kcal")

type CalorieAllocation struct {
	targetCalories float64
	proteinGrams   float64
	carbGrams      float64
	fatGrams       float64
}

func NewCalorieAllocation(targetCalories, proteinGrams, carbGrams, fatGrams float64) (CalorieAllocation, error) {
	if targetCalories < 1200.0 {
		return CalorieAllocation{}, fmt.Errorf("calorie allocation: %w", ErrInvalidCalories)
	}

	return CalorieAllocation{
		targetCalories: targetCalories,
		proteinGrams:   proteinGrams,
		carbGrams:      carbGrams,
		fatGrams:       fatGrams,
	}, nil
}

func (c CalorieAllocation) TargetCalories() float64 { return c.targetCalories }
func (c CalorieAllocation) ProteinGrams() float64   { return c.proteinGrams }
func (c CalorieAllocation) CarbGrams() float64      { return c.carbGrams }
func (c CalorieAllocation) FatGrams() float64       { return c.fatGrams }

func (c CalorieAllocation) RebalanceWithSurplus(surplusCalories float64) CalorieAllocation {
	newTarget := c.targetCalories + surplusCalories
	if newTarget < 1200.0 {
		newTarget = 1200.0
	}

	return CalorieAllocation{
		targetCalories: newTarget,
		proteinGrams:   c.proteinGrams + ((surplusCalories * 0.45) / 4.0),
		carbGrams:      c.carbGrams + ((surplusCalories * 0.45) / 4.0),
		fatGrams:       c.fatGrams + ((surplusCalories * 0.10) / 9.0),
	}
}
