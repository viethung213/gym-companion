package vo

import "errors"

var ErrInvalidNutritionGoal = errors.New("invalid nutrition goal")

// NutritionGoal biểu diễn mục tiêu dinh dưỡng dài hạn của người dùng.
// Ảnh hưởng đến tỷ lệ Macro (Protein/Carb/Fat) tính từ TDEE.
type NutritionGoal string

const (
	// GoalWeightLoss — giảm cân: ưu tiên Protein cao, Carb thấp, Calorie deficit.
	// Macro ratio: P 40% / C 30% / F 30%.
	GoalWeightLoss NutritionGoal = "WEIGHT_LOSS"
	// GoalMuscleGain — tăng cơ: ưu tiên Protein + Carb phức hợp, Calorie surplus nhẹ.
	// Macro ratio: P 35% / C 45% / F 20%.
	GoalMuscleGain NutritionGoal = "MUSCLE_GAIN"
	// GoalMaintenance — duy trì: cân bằng Macro, Calorie ngang TDEE.
	// Macro ratio: P 30% / C 40% / F 30%.
	GoalMaintenance NutritionGoal = "MAINTENANCE"
)

// MacroRatio trả về tỷ lệ phân bổ năng lượng (0.0–1.0) cho Protein, Carb, Fat
// tương ứng với mục tiêu dinh dưỡng.
func (g NutritionGoal) MacroRatio() (proteinRatio, carbRatio, fatRatio float64) {
	switch g {
	case GoalWeightLoss:
		return 0.40, 0.30, 0.30
	case GoalMuscleGain:
		return 0.35, 0.45, 0.20
	default:
		// GoalMaintenance và mọi giá trị không hợp lệ đều về mức duy trì an toàn.
		return 0.30, 0.40, 0.30
	}
}

// Validate kiểm tra NutritionGoal có giá trị hợp lệ không.
func (g NutritionGoal) Validate() error {
	switch g {
	case GoalWeightLoss, GoalMuscleGain, GoalMaintenance:
		return nil
	default:
		return ErrInvalidNutritionGoal
	}
}
