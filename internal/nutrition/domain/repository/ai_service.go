package repository

import (
	"context"
	"time"

	"github.com/viethung213/gym-companion/internal/nutrition/domain/aggregate"
	"github.com/viethung213/gym-companion/internal/nutrition/domain/vo"
)

// GeneratedRecipeResult là kết quả AI trả về cho 1 tùy chọn bữa ăn.
// TotalProteinGrams/TotalCarbGrams/TotalFatGrams là tổng macro AI đã tính sẵn.
type GeneratedRecipeResult struct {
	RecipeName               string
	CookingSteps             []string
	SupplementaryIngredients []aggregate.IngredientGram
	NewFoodCatalogItems      []vo.FoodNutrient
	TotalProteinGrams        float64
	TotalCarbGrams           float64
	TotalFatGrams            float64
}

// EstimatedNutrientResult là kết quả ước tính dinh dưỡng từ AI.
type EstimatedNutrientResult struct {
	Calories float64
	Protein  float64
	Carbs    float64
	Fat      float64
}

// AIMenuPromptContext là ngữ cảnh đầu vào cho AI khi sinh thực đơn.
// AI tự quyết định chọn combo thực phẩm, phân bổ Macro, và tính định lượng Gram.
type AIMenuPromptContext struct {
	UserID                   string
	MealType                 string
	NutritionGoal            string
	TargetMealCalories       float64
	TargetDailyCalories      float64
	TargetProteinGrams       float64
	TargetCarbGrams          float64
	TargetFatGrams           float64
	MandatoryUserIngredients []vo.FoodNutrient
	SupplementaryIngredients []vo.FoodNutrient
	AvailableIngredients     []vo.FoodNutrient
	UserRestrictions         []string
	PlanDate                 time.Time
	PreferredIngredients     []string // Nguyên liệu ưa chuộng từ đánh giá tuần trước (nếu có)
	WeeklyCookingStyle       string   // Phong cách nấu ăn thường xuyên từ tuần trước (nếu có)
}

// WeeklyMacroSummary thống kê dinh dưỡng tinh gọn tổng hợp của 7 ngày.
type WeeklyMacroSummary struct {
	TotalConsumedCalories float64 `json:"total_consumed_calories"`
	TotalTargetCalories   float64 `json:"total_target_calories"`
	TotalConsumedProtein  float64 `json:"total_consumed_protein"`
	TotalTargetProtein    float64 `json:"total_target_protein"`
	TotalConsumedCarbs    float64 `json:"total_consumed_carbs"`
	TotalTargetCarbs      float64 `json:"total_target_carbs"`
	TotalConsumedFat      float64 `json:"total_consumed_fat"`
	TotalTargetFat        float64 `json:"total_target_fat"`
	LoggedMealsCount      int     `json:"logged_meals_count"`
	TotalExpectedMeals    int     `json:"total_expected_meals"`
}

// DailyMealSlotRecord ghi nhận các món ăn trong 1 ngày theo 4 bữa. Bữa nào không ghi để trống "".
type DailyMealSlotRecord struct {
	Date      string `json:"date"`
	Breakfast string `json:"breakfast"`
	Lunch     string `json:"lunch"`
	Dinner    string `json:"dinner"`
	Snack     string `json:"snack"`
}

// ImprovementArea mô tả khu vực cần tăng hoặc giảm lượng chất trong tuần do AI phân tích.
type ImprovementArea struct {
	Nutrient    string  `json:"nutrient"`     // "CALORIES", "PROTEIN", "CARBS", "FAT"
	Action      string  `json:"action"`       // "INCREASE", "DECREASE", "MAINTAIN"
	DeltaAmount float64 `json:"delta_amount"` // Lượng tăng/giảm cần điều chỉnh
	Suggestion  string  `json:"suggestion"`   // Hướng dẫn cụ thể
}

// NutritionInsightResult là kết quả phân tích insight từ AI.
type NutritionInsightResult struct {
	OverallScore          int               `json:"overall_score"`
	Summary               string            `json:"summary"`
	ImprovementAreas      []ImprovementArea `json:"improvement_areas"`
	PredictedCookingStyle string            `json:"predicted_cooking_style"`
	PreferredIngredients  []string          `json:"preferred_ingredients"`
}

// InsightPromptContext là ngữ cảnh đầu vào cho AI khi phân tích insight dinh dưỡng 7 ngày.
type InsightPromptContext struct {
	UserID             string                `json:"user_id"`
	GoalType           string                `json:"goal_type"` // "WEIGHT_LOSS", "MUSCLE_GAIN", "MAINTENANCE"
	WeeklyMacroSummary WeeklyMacroSummary    `json:"weekly_macro_summary"`
	WeeklyMealSlots    []DailyMealSlotRecord `json:"weekly_meal_slots"`
}

// AIService là port interface cho AI Agent trong module Nutrition.
type AIService interface {
	// SelectCreativeMealOptions sinh 1-3 tùy chọn bữa ăn sáng tạo cho 1 slot bữa.
	// AI tự chọn combo, tự tính Gram Macro theo TDEE mục tiêu.
	SelectCreativeMealOptions(
		ctx context.Context,
		promptCtx AIMenuPromptContext,
	) ([]GeneratedRecipeResult, error)

	// EstimateNutrient ước tính dinh dưỡng khi người dùng nhập thủ công tên món.
	EstimateNutrient(
		ctx context.Context,
		mealName, portion string,
	) (*EstimatedNutrientResult, error)

	// GenerateNutritionInsight phân tích lịch sử dinh dưỡng và trả về hướng cải thiện.
	GenerateNutritionInsight(
		ctx context.Context,
		promptCtx InsightPromptContext,
	) (*NutritionInsightResult, error)
}
