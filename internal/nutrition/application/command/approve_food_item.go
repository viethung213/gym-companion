package command

import (
	"context"
	"fmt"

	"github.com/viethung213/gym-companion/internal/nutrition/domain/aggregate"
	"github.com/viethung213/gym-companion/internal/nutrition/domain/repository"
)

type ApproveFoodItemCommand struct {
	FoodItemID string
}

type ApproveFoodItemHandler struct {
	foodRepo repository.FoodItemRepository
}

func NewApproveFoodItemHandler(foodRepo repository.FoodItemRepository) *ApproveFoodItemHandler {
	return &ApproveFoodItemHandler{foodRepo: foodRepo}
}

func (h *ApproveFoodItemHandler) Handle(ctx context.Context, cmd ApproveFoodItemCommand) (*aggregate.FoodItem, error) {
	item, err := h.foodRepo.FindByID(ctx, cmd.FoodItemID)
	if err != nil || item == nil {
		return nil, fmt.Errorf("approve food item: not found: %w", err)
	}

	if err := item.Approve(); err != nil {
		return nil, fmt.Errorf("approve food item domain: %w", err)
	}

	if err := h.foodRepo.Update(ctx, item); err != nil {
		return nil, fmt.Errorf("approve food item update: %w", err)
	}

	return item, nil
}
