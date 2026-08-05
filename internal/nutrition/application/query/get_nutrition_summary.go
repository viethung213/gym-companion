package query

import (
	"context"
	"time"

	"github.com/viethung213/gym-companion/internal/nutrition/domain/repository"
)

type GetNutritionSummaryQuery struct {
	UserID   string
	PlanDate time.Time
}

type NutritionSummaryResult struct {
	TargetCalories   float64
	TargetProtein    float64
	TargetCarbs      float64
	TargetFat        float64
	ConsumedCalories float64
	ConsumedProtein  float64
	ConsumedCarbs    float64
	ConsumedFat      float64
}

type GetNutritionSummaryHandler struct {
	planRepo    repository.NutritionPlanRepository
	historyRepo repository.MealHistoryRepository
}

func NewGetNutritionSummaryHandler(
	planRepo repository.NutritionPlanRepository,
	historyRepo repository.MealHistoryRepository,
) *GetNutritionSummaryHandler {
	return &GetNutritionSummaryHandler{
		planRepo:    planRepo,
		historyRepo: historyRepo,
	}
}

func (h *GetNutritionSummaryHandler) Handle(ctx context.Context, q GetNutritionSummaryQuery) (*NutritionSummaryResult, error) {
	result := &NutritionSummaryResult{}

	plan, planErr := h.planRepo.FindByUserIDAndDate(ctx, q.UserID, q.PlanDate)
	if planErr == nil && plan != nil {
		alloc := plan.CalorieAllocation()
		result.TargetCalories = alloc.TargetCalories()
		result.TargetProtein = alloc.ProteinGrams()
		result.TargetCarbs = alloc.CarbGrams()
		result.TargetFat = alloc.FatGrams()
	}

	history, histErr := h.historyRepo.FindByUserID(ctx, q.UserID)
	if histErr == nil && history != nil {
		for _, log := range history.MealLogs() {
			if log.LoggedAt().Year() == q.PlanDate.Year() &&
				log.LoggedAt().YearDay() == q.PlanDate.YearDay() {
				result.ConsumedCalories += log.Calories()
				result.ConsumedProtein += log.Protein()
				result.ConsumedCarbs += log.Carbs()
				result.ConsumedFat += log.Fat()
			}
		}
	}

	return result, nil
}
