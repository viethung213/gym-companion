package command

import (
	"context"
	"fmt"
	"time"

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
}

type CreateFoodItemHandler struct {
	foodRepo repository.FoodItemRepository
}

func NewCreateFoodItemHandler(foodRepo repository.FoodItemRepository) *CreateFoodItemHandler {
	return &CreateFoodItemHandler{foodRepo: foodRepo}
}

func (h *CreateFoodItemHandler) Handle(ctx context.Context, cmd CreateFoodItemCommand) (*aggregate.FoodItem, error) {
	item := aggregate.ReconstructFoodItem(
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
		aggregate.FoodStatusPendingApproval,
		time.Now(),
		time.Now(),
	)

	if err := h.foodRepo.Save(ctx, item); err != nil {
		return nil, fmt.Errorf("create food item save: %w", err)
	}

	return item, nil
}
