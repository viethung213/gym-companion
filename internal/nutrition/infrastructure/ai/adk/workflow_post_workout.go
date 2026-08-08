package adk

import (
	"context"

	"github.com/viethung213/gym-companion/internal/nutrition/domain/repository"
	"github.com/viethung213/gym-companion/internal/nutrition/domain/vo"
)

// ExecutePostWorkoutRecalibrationWorkflow runs the post-workout calorie rebalancing workflow.
//
//nolint:gocritic // promptCtx parameter struct is passed by value
func (a *NutritionAgent) ExecutePostWorkoutRecalibrationWorkflow(
	ctx context.Context,
	promptCtx NutritionPromptContext,
	_ float64,
	lockoutRegistry vo.LockoutRegistry,
) ([]repository.GeneratedRecipeResult, error) {
	domainFoods := ToFoodNutrientDomains(promptCtx.AvailableIngredients)
	filteredFoods := lockoutRegistry.FilterAvailableIngredients(domainFoods, promptCtx.PlanDate)
	availableDTOs := ToFoodNutrientDTOs(filteredFoods)
	plan, err := a.runPostWorkoutWorkflow(ctx, promptCtx.UserID, availableDTOs)
	if err != nil {
		return nil, err
	}
	return a.persistNewFoodItemsAndMap(ctx, plan)
}
