package adk

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/viethung213/gym-companion/internal/nutrition/domain/repository"
	"github.com/viethung213/gym-companion/internal/nutrition/domain/vo"
)

type NutritionTools struct {
	foodRepo repository.FoodItemRepository
}

func NewNutritionTools(foodRepo repository.FoodItemRepository) *NutritionTools {
	return &NutritionTools{foodRepo: foodRepo}
}

func (t *NutritionTools) FetchActiveFoodCatalog(ctx context.Context, category string) ([]vo.FoodNutrient, error) {
	catalog, err := t.foodRepo.FindActiveCatalog(ctx)
	if err != nil {
		return nil, fmt.Errorf("tools fetch active catalog: %w", err)
	}

	if category == "" {
		return catalog, nil
	}

	filtered := make([]vo.FoodNutrient, 0, len(catalog))
	for _, item := range catalog {
		if strings.EqualFold(item.Category(), category) {
			filtered = append(filtered, item)
		}
	}
	return filtered, nil
}

func (t *NutritionTools) CheckLockoutRules(
	_ context.Context,
	lockoutRegistry vo.LockoutRegistry,
	ingredients []string,
) (collisions []string, warnings []string) {
	return lockoutRegistry.CheckCollisions(ingredients, time.Now())
}

func (t *NutritionTools) CalculateMacroGramRatio(
	targetCalories float64,
	proteinRatio, carbRatio, fatRatio float64,
) (proteinGrams, carbGrams, fatGrams float64) {
	if proteinRatio <= 0 {
		proteinRatio = 0.45
	}
	if carbRatio <= 0 {
		carbRatio = 0.45
	}
	if fatRatio <= 0 {
		fatRatio = 0.10
	}

	proteinGrams = (targetCalories * proteinRatio) / 4.0
	carbGrams = (targetCalories * carbRatio) / 4.0
	fatGrams = (targetCalories * fatRatio) / 9.0
	return proteinGrams, carbGrams, fatGrams
}

func (t *NutritionTools) SuggestNutiFoodSupplement(ctx context.Context) ([]vo.FoodNutrient, error) {
	products, err := t.foodRepo.FindNutiFoodProducts(ctx)
	if err != nil {
		return nil, fmt.Errorf("tools suggest nutifood products: %w", err)
	}
	return products, nil
}
