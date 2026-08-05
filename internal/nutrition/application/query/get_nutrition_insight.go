package query

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/viethung213/gym-companion/internal/nutrition/domain/repository"
)

// GetNutritionInsightQuery yêu cầu AI phân tích xu hướng dinh dưỡng N ngày gần nhất.
type GetNutritionInsightQuery struct {
	UserID    string
	GoalType  string // "WEIGHT_LOSS", "MUSCLE_GAIN", "MAINTENANCE"
	RangeDays int    // số ngày cần phân tích, mặc định 7
}

// GetNutritionInsightHandler xử lý GetNutritionInsightQuery.
type GetNutritionInsightHandler struct {
	planRepo    repository.NutritionPlanRepository
	historyRepo repository.MealHistoryRepository
	aiService   repository.AIService
}

// NewGetNutritionInsightHandler khởi tạo GetNutritionInsightHandler.
func NewGetNutritionInsightHandler(
	planRepo repository.NutritionPlanRepository,
	historyRepo repository.MealHistoryRepository,
	aiService repository.AIService,
) *GetNutritionInsightHandler {
	return &GetNutritionInsightHandler{
		planRepo:    planRepo,
		historyRepo: historyRepo,
		aiService:   aiService,
	}
}

// Handle tổng hợp dữ liệu N ngày rồi gọi AI để sinh insight dinh dưỡng.
func (h *GetNutritionInsightHandler) Handle(
	ctx context.Context,
	q GetNutritionInsightQuery,
) (*repository.NutritionInsightResult, error) {
	if q.RangeDays <= 0 {
		q.RangeDays = 7
	}

	history, err := h.historyRepo.FindByUserID(ctx, q.UserID)
	if err != nil || history == nil {
		return nil, fmt.Errorf("get nutrition insight: meal history not found: %w", err)
	}

	now := time.Now()
	dailyData := make([]repository.DailyNutritionData, 0, q.RangeDays)

	for i := q.RangeDays - 1; i >= 0; i-- {
		day := now.AddDate(0, 0, -i)
		consumed, protein, carbs, fat := history.CalculateConsumedToday(day)

		// Lấy target từ plan của ngày đó nếu có.
		var targetCal, targetProt, targetCarbs, targetFat float64
		plan, planErr := h.planRepo.FindByUserIDAndDate(ctx, q.UserID, day)
		if planErr == nil && plan != nil {
			alloc := plan.CalorieAllocation()
			targetCal = alloc.TargetCalories()
			targetProt = alloc.ProteinGrams()
			targetCarbs = alloc.CarbGrams()
			targetFat = alloc.FatGrams()
		}

		// Đếm số bữa đã log trong ngày.
		mealCount := 0
		logs := history.MealLogs()
		for i := range logs {
			y, m, d := logs[i].LoggedAt().Date()
			dy, dm, dd := day.Date()
			if y == dy && m == dm && d == dd {
				mealCount++
			}
		}

		dailyData = append(dailyData, repository.DailyNutritionData{
			Date:             day,
			ConsumedCalories: consumed,
			ConsumedProtein:  protein,
			ConsumedCarbs:    carbs,
			ConsumedFat:      fat,
			TargetCalories:   targetCal,
			TargetProtein:    targetProt,
			TargetCarbs:      targetCarbs,
			TargetFat:        targetFat,
			MealCount:        mealCount,
		})
	}

	if h.aiService == nil {
		return nil, errors.New("get nutrition insight: ai service not configured")
	}

	promptCtx := repository.InsightPromptContext{
		UserID:    q.UserID,
		GoalType:  q.GoalType,
		DailyData: dailyData,
		RangeDays: q.RangeDays,
	}

	insight, err := h.aiService.GenerateNutritionInsight(ctx, promptCtx)
	if err != nil {
		return nil, fmt.Errorf("get nutrition insight: ai generate: %w", err)
	}

	return insight, nil
}
