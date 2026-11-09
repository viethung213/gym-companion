package adk

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/google/uuid"
	"google.golang.org/adk/v2/agent"
	"google.golang.org/adk/v2/runner"
	"google.golang.org/genai"
)

// runWorkflow chạy một workflow agent với NutritionPromptContext cho trước,
// thu thập GeneratedMealPlan từ node kết quả và trả về.
//
//nolint:gocritic // hugeParam: pCtx holds prompt context
func (a *NutritionAgent) runWorkflow(
	ctx context.Context,
	wfAgent agent.Agent,
	pCtx NutritionPromptContext,
	fb ...*attemptFeedback,
) (*GeneratedMealPlan, error) {
	r, err := runner.NewInMemory("nutrition-app", wfAgent)
	if err != nil {
		return nil, fmt.Errorf("new runner: %w", err)
	}

	sessionID := uuid.NewString()
	defer func() {
		a.mu.Lock()
		delete(a.results, sessionID)
		a.mu.Unlock()
	}()

	if a.tools != nil {
		a.tools.RegisterSessionContext(sessionID, SessionFilterContext{
			UserID:           pCtx.UserID,
			LockoutRegistry:  pCtx.LockoutRegistry,
			UserRestrictions: pCtx.UserRestrictions,
			PlanDate:         pCtx.PlanDate,
		})
		defer a.tools.UnregisterSessionContext(sessionID)
	}

	// Encode input context cho workflow agent kèm feedback của vòng lặp trước nếu có.
	inputMap := map[string]any{
		"user_id":                     pCtx.UserID,
		"flow":                        pCtx.MealType,
		"meal_type":                   pCtx.MealType,
		"nutrition_goal":              pCtx.NutritionGoal,
		"target_meal_calories":        pCtx.TargetMealCalories,
		"target_daily_calories":       pCtx.TargetDailyCalories,
		"target_protein_grams":        pCtx.TargetProteinGrams,
		"target_carb_grams":           pCtx.TargetCarbGrams,
		"target_fat_grams":            pCtx.TargetFatGrams,
		"user_restrictions":           pCtx.UserRestrictions,
		"mandatory_user_ingredients":  pCtx.MandatoryUserIngredients,
		"supplementary_db_candidates": pCtx.SupplementaryIngredients,
		"available_ingredients":       pCtx.AvailableIngredients,
	}
	if len(fb) > 0 && fb[0] != nil {
		if len(fb[0].Issues) > 0 {
			inputMap["prior_attempt_issues"] = fb[0].Issues
		}
		if fb[0].Review != nil {
			inputMap["review_feedback"] = fb[0].Review
		}
		if fb[0].PreviousPlan != nil {
			inputMap["previous_plan"] = fb[0].PreviousPlan
		}
	}

	inputBytes, err := json.Marshal(inputMap)
	if err != nil {
		return nil, fmt.Errorf("marshal workflow input: %w", err)
	}

	prompt := &genai.Content{
		Role:  "user",
		Parts: []*genai.Part{{Text: string(inputBytes)}},
	}

	for _, runErr := range r.Run(ctx, pCtx.UserID, sessionID, prompt, agent.RunConfig{}) {
		if runErr != nil {
			return nil, fmt.Errorf("runner step: %w", runErr)
		}
	}

	plan, ok := a.getResult(sessionID)
	if !ok || plan == nil || len(plan.Options) == 0 {
		return nil, fmt.Errorf("%w: workflow produced no meal options", ErrNutritionPlanFailed)
	}
	return plan, nil
}

// runInitWorkflow là helper cho luồng 5:00 AM.
//
//nolint:gocritic // hugeParam: pCtx holds prompt context
func (a *NutritionAgent) runInitWorkflow(ctx context.Context, pCtx NutritionPromptContext) (*GeneratedMealPlan, error) {
	pCtx.MealType = FlowDaily
	return a.runWorkflow(ctx, a.dailyWorkflowAgent, pCtx)
}

// runPostWorkoutWorkflow là helper cho luồng bù Calo sau buổi tập.
//
//nolint:gocritic // hugeParam: pCtx holds prompt context
func (a *NutritionAgent) runPostWorkoutWorkflow(ctx context.Context, pCtx NutritionPromptContext) (*GeneratedMealPlan, error) {
	pCtx.MealType = FlowPostWorkout
	return a.runWorkflow(ctx, a.postWorkoutWorkflowAgent, pCtx)
}

// runPantryWorkflow là helper cho luồng chế biến từ tủ lạnh.
//
//nolint:gocritic // hugeParam: pCtx holds prompt context
func (a *NutritionAgent) runPantryWorkflow(ctx context.Context, pCtx NutritionPromptContext) (*GeneratedMealPlan, error) {
	pCtx.MealType = FlowPantry
	return a.runWorkflow(ctx, a.pantryWorkflowAgent, pCtx)
}

// runAdhocWorkflow là helper cho luồng gợi ý bữa nhanh.
//
//nolint:gocritic // hugeParam: pCtx holds prompt context
func (a *NutritionAgent) runAdhocWorkflow(ctx context.Context, pCtx NutritionPromptContext) (*GeneratedMealPlan, error) {
	pCtx.MealType = FlowAdhoc
	return a.runWorkflow(ctx, a.adhocWorkflowAgent, pCtx)
}
