package query

import (
	"context"
	"fmt"
	"time"

	"github.com/viethung213/gym-companion/internal/nutrition/domain/aggregate"
	"github.com/viethung213/gym-companion/internal/nutrition/domain/repository"
)

type GetTodayMenuQuery struct {
	UserID   string
	PlanDate time.Time
}

type GetTodayMenuHandler struct {
	planRepo repository.NutritionPlanRepository
}

func NewGetTodayMenuHandler(planRepo repository.NutritionPlanRepository) *GetTodayMenuHandler {
	return &GetTodayMenuHandler{planRepo: planRepo}
}

func (h *GetTodayMenuHandler) Handle(ctx context.Context, q GetTodayMenuQuery) (*aggregate.NutritionPlan, error) {
	plan, err := h.planRepo.FindByUserIDAndDate(ctx, q.UserID, q.PlanDate)
	if err != nil {
		return nil, fmt.Errorf("get today menu query: %w", err)
	}
	return plan, nil
}
