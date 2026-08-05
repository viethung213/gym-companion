package query

import (
	"context"
	"fmt"
	"time"

	"github.com/viethung213/gym-companion/internal/nutrition/domain/aggregate"
	"github.com/viethung213/gym-companion/internal/nutrition/domain/repository"
)

type GetNutritionHistoryQuery struct {
	UserID    string
	StartDate time.Time
	EndDate   time.Time
}

type GetNutritionHistoryHandler struct {
	historyRepo repository.MealHistoryRepository
}

func NewGetNutritionHistoryHandler(historyRepo repository.MealHistoryRepository) *GetNutritionHistoryHandler {
	return &GetNutritionHistoryHandler{historyRepo: historyRepo}
}

func (h *GetNutritionHistoryHandler) Handle(ctx context.Context, q GetNutritionHistoryQuery) (*aggregate.MealHistory, error) {
	history, err := h.historyRepo.FindByUserID(ctx, q.UserID)
	if err != nil {
		return nil, fmt.Errorf("get nutrition history query: %w", err)
	}
	return history, nil
}
