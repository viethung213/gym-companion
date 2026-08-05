package adk

import (
	"context"

	"github.com/viethung213/gym-companion/internal/nutrition/domain/repository"
	"github.com/viethung213/gym-companion/internal/nutrition/domain/vo"
)

// ExecuteAdhocMealSuggestionWorkflow runs the quick standalone meal suggestion workflow.
func (a *NutritionAgent) ExecuteAdhocMealSuggestionWorkflow(
	ctx context.Context,
	promptCtx NutritionPromptContext,
	lockoutRegistry vo.LockoutRegistry,
) ([]repository.GeneratedRecipeResult, error) {
	availableFoods := lockoutRegistry.FilterAvailableIngredients(promptCtx.AvailableIngredients, promptCtx.PlanDate)
	plan, err := a.runAdhocWorkflow(ctx, promptCtx.UserID, availableFoods)
	if err != nil {
		return nil, err
	}
	return a.persistNewFoodItemsAndMap(ctx, plan)
}
