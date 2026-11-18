package adk

import (
	"context"

	"github.com/viethung213/gym-companion/internal/nutrition/domain/repository"
)

// ExecuteDailyMenuWorkflow runs the 5:00 AM full day 4-meal generation workflow.
//
//nolint:gocritic // promptCtx parameter struct is passed by value
func (a *NutritionAgent) ExecuteDailyMenuWorkflow(
	ctx context.Context,
	promptCtx NutritionPromptContext,
) ([]repository.GeneratedRecipeResult, error) {
	plan, err := a.runInitWorkflow(ctx, promptCtx)
	if err != nil {
		return nil, err
	}
	return a.persistNewFoodItemsAndMap(ctx, plan, promptCtx.AvailableIngredients)
}
