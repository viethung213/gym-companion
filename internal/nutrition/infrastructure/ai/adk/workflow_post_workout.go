package adk

import (
	"context"

	"github.com/viethung213/gym-companion/internal/nutrition/domain/repository"
)

// ExecutePostWorkoutRecalibrationWorkflow runs the post-workout calorie rebalancing workflow.
//
//nolint:gocritic // promptCtx parameter struct is passed by value
func (a *NutritionAgent) ExecutePostWorkoutRecalibrationWorkflow(
	ctx context.Context,
	promptCtx NutritionPromptContext,
	_ float64,
) ([]repository.GeneratedRecipeResult, error) {
	plan, err := a.runPostWorkoutWorkflow(ctx, promptCtx)
	if err != nil {
		return nil, err
	}
	return a.persistNewFoodItemsAndMap(ctx, plan, promptCtx.AvailableIngredients)
}
