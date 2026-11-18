package adk

import (
	"context"

	"github.com/viethung213/gym-companion/internal/nutrition/domain/repository"
)

// ExecutePantryRecipeWorkflow runs the meal creation workflow using available pantry ingredients.
//
//nolint:gocritic // promptCtx parameter struct is passed by value
func (a *NutritionAgent) ExecutePantryRecipeWorkflow(
	ctx context.Context,
	promptCtx NutritionPromptContext,
	_ []string,
) ([]repository.GeneratedRecipeResult, error) {
	plan, err := a.runPantryWorkflow(ctx, promptCtx)
	if err != nil {
		return nil, err
	}
	return a.persistNewFoodItemsAndMap(ctx, plan, promptCtx.AvailableIngredients)
}
