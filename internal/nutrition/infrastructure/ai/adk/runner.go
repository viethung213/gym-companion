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
		a.tools.RegisterSessionContext(sessionID, &SessionFilterContext{
			UserID:           pCtx.UserID,
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
	if pCtx.MealType == FlowPantry || len(pCtx.MandatoryUserIngredients) > 0 {
		inputMap["critical_pantry_instruction"] = "CRITICAL MANDATORY PANTRY RULES:\n" +
			"1. You MUST feature and cook with all items from 'mandatory_user_ingredients' in EVERY meal option (Option 1 AND Option 2).\n" +
			"2. For any item with category 'UNKNOWN': You (AI) MUST analyze and determine its food group (e.g. 'thịt cóc' / 'thịt nai' / 'ếch' is PROTEIN; " +
			"'khoai mỡ' is CARB; 'rau dền' is VEGGIE).\n" +
			"3. Once determined, that group is ALREADY FULFILLED! You MUST use it in that role (protein_food_name and protein_food_id) in ALL options. " +
			"STRICTLY FORBIDDEN to pick duplicate foods from DB for that group (e.g. DO NOT pick Pork, Chicken, Beef, Fish, or Eggs from DB when user provided " +
			"'thịt cóc' or 'thịt nai').\n" +
			"4. NOVEL & EXOTIC INGREDIENTS (e.g. 'thịt cóc' / toad meat): You MUST treat it as a valid protein source and cook with it. " +
			"NEVER drop, omit, or replace it with DB items!\n" +
			"5. Set the respective food ID (protein_food_id) to the EXACT 'id' string given in 'mandatory_user_ingredients'.\n" +
			"6. Diversity across Option 1 and Option 2 must come from pairing with DIFFERENT Carbs, DIFFERENT Veggies, and DIFFERENT cooking methods, " +
			"NOT from swapping the user's mandatory ingredient.\n" +
			"7. Estimate realistic macros per 100g for UNKNOWN ingredients and calculate accurate meal totals."
	}
	if len(pCtx.PreferredIngredients) > 0 {
		inputMap["preferred_ingredients"] = pCtx.PreferredIngredients
	}
	if pCtx.WeeklyCookingStyle != "" {
		inputMap["weekly_cooking_style"] = pCtx.WeeklyCookingStyle
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

// runPantryWorkflow là helper cho luồng chế biến từ tủ lạnh.
//
//nolint:gocritic // hugeParam: pCtx holds prompt context
func (a *NutritionAgent) runPantryWorkflow(ctx context.Context, pCtx NutritionPromptContext) (*GeneratedMealPlan, error) {
	pCtx.MealType = FlowPantry
	return a.runWorkflow(ctx, a.pantryWorkflowAgent, pCtx)
}
