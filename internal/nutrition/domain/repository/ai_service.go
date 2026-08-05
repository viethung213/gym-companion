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
	UserID               string
	MealType             string
	TargetMealCalories   float64
	AvailableIngredients []vo.FoodNutrient
	UserRestrictions     []string
	PlanDate             time.Time
}

// DailyNutritionData chứa dữ liệu dinh dưỡng tổng hợp của một ngày.
type DailyNutritionData struct {
	Date             time.Time
	ConsumedCalories float64
	ConsumedProtein  float64
	ConsumedCarbs    float64
	ConsumedFat      float64
	TargetCalories   float64
	TargetProtein    float64
	TargetCarbs      float64
	TargetFat        float64
	MealCount        int
}

// ImprovementArea mô tả một khu vực cần cải thiện do AI phân tích.
type ImprovementArea struct {
	Area        string  // ví dụ: "Protein Intake", "Meal Timing", "Hydration"
	CurrentAvg  float64 // giá trị trung bình hiện tại
	Target      float64 // giá trị mục tiêu
	Suggestion  string  // gợi ý hành động cụ thể
	Priority    string  // HIGH, MEDIUM, LOW
}

// RecommendedAdjustments là các điều chỉnh AI đề xuất cho kế hoạch dinh dưỡng.
type RecommendedAdjustments struct {
	CaloriesDelta      float64  // dương = tăng, âm = giảm
	ProteinRatioDelta  float64
	FocusFoods         []string
}

// NutritionInsightResult là kết quả phân tích insight từ AI.
type NutritionInsightResult struct {
	OverallScore           int
	Summary                string
	Strengths              []string
	ImprovementAreas       []ImprovementArea
	WeeklyTrend            string // "IMPROVING", "DECLINING", "STABLE"
	RecommendedAdjustments RecommendedAdjustments
}

// InsightPromptContext là ngữ cảnh đầu vào cho AI khi phân tích insight dinh dưỡng.
type InsightPromptContext struct {
	UserID      string
	GoalType    string // "WEIGHT_LOSS", "MUSCLE_GAIN", "MAINTENANCE"
	DailyData   []DailyNutritionData
	RangeDays   int
}

// AIService là port interface cho AI Agent trong module Nutrition.
type AIService interface {
	// SelectCreativeMealOptions sinh 1-3 tùy chọn bữa ăn sáng tạo cho 1 slot bữa.
	// AI tự chọn combo, tự tính Gram Macro theo TDEE mục tiêu.
	SelectCreativeMealOptions(
		ctx context.Context,
		promptCtx AIMenuPromptContext,
		lockoutRegistry vo.LockoutRegistry,
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
