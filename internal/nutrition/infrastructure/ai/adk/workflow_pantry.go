package adk

import (
	"context"

	"github.com/viethung213/gym-companion/internal/nutrition/domain/repository"
	"github.com/viethung213/gym-companion/internal/nutrition/domain/vo"
)

// ExecutePantryRecipeWorkflow runs the meal creation workflow using available pantry ingredients.
//
//nolint:gocritic // promptCtx parameter struct is passed by value
func (a *NutritionAgent) ExecutePantryRecipeWorkflow(
	ctx context.Context,
	promptCtx NutritionPromptContext,
	_ []string,
	lockoutRegistry vo.LockoutRegistry,
) ([]repository.GeneratedRecipeResult, error) {
	availableFoods := lockoutRegistry.FilterAvailableIngredients(promptCtx.AvailableIngredients, promptCtx.PlanDate)
	plan, err := a.runPantryWorkflow(ctx, promptCtx.UserID, availableFoods)
	if err != nil {
		return nil, err
	}
	return a.persistNewFoodItemsAndMap(ctx, plan)
}
