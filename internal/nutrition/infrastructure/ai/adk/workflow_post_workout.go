package adk

import (
	"context"

	"github.com/viethung213/gym-companion/internal/nutrition/domain/repository"
	"github.com/viethung213/gym-companion/internal/nutrition/domain/vo"
)

// ExecutePostWorkoutRecalibrationWorkflow runs the post-workout calorie rebalancing workflow.
func (a *ADKNutritionAgent) ExecutePostWorkoutRecalibrationWorkflow(
	ctx context.Context,
	promptCtx NutritionPromptContext,
	_ float64,
	lockoutRegistry vo.LockoutRegistry,
) ([]repository.GeneratedRecipeResult, error) {
	availableFoods := lockoutRegistry.FilterAvailableIngredients(promptCtx.AvailableIngredients, promptCtx.PlanDate)
	plan, err := a.runPostWorkoutWorkflow(ctx, promptCtx.UserID, availableFoods)
	if err != nil {
		return nil, err
	}
	return a.persistNewFoodItemsAndMap(ctx, plan)
}
