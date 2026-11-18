package adk

import (
	"context"

	"github.com/viethung213/gym-companion/internal/nutrition/domain/repository"
)

// ExecuteAdhocMealSuggestionWorkflow runs the quick standalone meal suggestion workflow.
//
//nolint:gocritic // promptCtx parameter struct is passed by value
func (a *NutritionAgent) ExecuteAdhocMealSuggestionWorkflow(
	ctx context.Context,
	promptCtx NutritionPromptContext,
) ([]repository.GeneratedRecipeResult, error) {
	plan, err := a.runAdhocWorkflow(ctx, promptCtx)
	if err != nil {
		return nil, err
	}
	return a.persistNewFoodItemsAndMap(ctx, plan, promptCtx.AvailableIngredients)
}
