package command

import (
	"context"
	"fmt"

	"github.com/google/uuid"
	"github.com/viethung213/gym-companion/internal/nutrition/domain/aggregate"
	"github.com/viethung213/gym-companion/internal/nutrition/domain/repository"
)

type CreateFoodItemCommand struct {
	Name              string
	Category          string
	CaloriesPer100g   float64
	ProteinPer100g    float64
	CarbsPer100g      float64
	FatPer100g        float64
	AllergenTags      []string
	ProteinSource     string
	CarbSource        string
	IsNutiFoodProduct bool
	PricePer100g      float64
	CostTier          string
}

type CreateFoodItemHandler struct {
	foodRepo repository.FoodItemRepository
}

func NewCreateFoodItemHandler(foodRepo repository.FoodItemRepository) *CreateFoodItemHandler {
	return &CreateFoodItemHandler{foodRepo: foodRepo}
}

//nolint:gocritic // cmd is passed by value per command pattern
func (h *CreateFoodItemHandler) Handle(ctx context.Context, cmd CreateFoodItemCommand) (*aggregate.FoodItem, error) {
	item, err := aggregate.NewFoodItem(
		uuid.New().String(),
		cmd.Name,
		cmd.Category,
		cmd.CaloriesPer100g,
		cmd.ProteinPer100g,
		cmd.CarbsPer100g,
		cmd.FatPer100g,
		cmd.AllergenTags,
		cmd.ProteinSource,
		cmd.CarbSource,
		cmd.IsNutiFoodProduct,
		cmd.PricePer100g,
		cmd.CostTier,
	)
	if err != nil {
		return nil, fmt.Errorf("create food item: %w", err)
	}

	if err := item.SubmitForApproval(); err != nil {
		return nil, fmt.Errorf("submit food item for approval: %w", err)
	}

	if err := h.foodRepo.Save(ctx, item); err != nil {
		return nil, fmt.Errorf("create food item save: %w", err)
	}

	return item, nil
}
