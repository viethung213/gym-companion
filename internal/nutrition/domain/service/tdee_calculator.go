package service

import (
	"strings"

	"github.com/viethung213/gym-companion/internal/nutrition/domain/vo"
)

// BiologicalMetrics chứa các chỉ số sinh học của người dùng để tính TDEE.
type BiologicalMetrics struct {
	WeightKg      float64
	HeightCm      float64
	Age           int
	Gender        string // MALE, FEMALE
	ActivityLevel string // SEDENTARY, LIGHTLY_ACTIVE, MODERATELY_ACTIVE, VERY_ACTIVE
}

// TDEECalculator tính Total Daily Energy Expenditure bằng công thức Mifflin-St Jeor.
type TDEECalculator struct{}

// NewTDEECalculator khởi tạo TDEECalculator.
func NewTDEECalculator() *TDEECalculator {
	return &TDEECalculator{}
}

// CalculateBaseTDEE tính TDEE cơ sở và phân bổ Macro theo NutritionGoal.
// Nếu goal không hợp lệ, mặc định về GoalMaintenance (30/40/30).
func (t *TDEECalculator) CalculateBaseTDEE(metrics BiologicalMetrics, goal vo.NutritionGoal) (vo.CalorieAllocation, error) {
	var bmr float64
	genderUpper := strings.ToUpper(metrics.Gender)

	if genderUpper == "FEMALE" {
		bmr = (10.0 * metrics.WeightKg) + (6.25 * metrics.HeightCm) - (5.0 * float64(metrics.Age)) - 161.0
	} else {
		bmr = (10.0 * metrics.WeightKg) + (6.25 * metrics.HeightCm) - (5.0 * float64(metrics.Age)) + 5.0
	}

	activityMultiplier := 1.2
	switch strings.ToUpper(metrics.ActivityLevel) {
	case "LIGHTLY_ACTIVE":
		activityMultiplier = 1.375
	case "MODERATELY_ACTIVE":
		activityMultiplier = 1.55
	case "VERY_ACTIVE":
		activityMultiplier = 1.725
	}

	baseCalories := bmr * activityMultiplier
	if baseCalories < 1200.0 {
		baseCalories = 1200.0 // BR-NU-01 Minimum Safety Requirement
	}

	proteinRatio, carbRatio, fatRatio := goal.MacroRatio()
	proteinGrams := (baseCalories * proteinRatio) / 4.0
	carbGrams := (baseCalories * carbRatio) / 4.0
	fatGrams := (baseCalories * fatRatio) / 9.0

	return vo.NewCalorieAllocation(baseCalories, proteinGrams, carbGrams, fatGrams)
}
