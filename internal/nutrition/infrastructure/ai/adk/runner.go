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

// runWorkflow chạy một workflow agent với userID và flow type cho trước,
// thu thập GeneratedMealPlan từ node kết quả và trả về.
func (a *NutritionAgent) runWorkflow(
	ctx context.Context,
	wfAgent agent.Agent,
	userID, flow string,
	availableFoods interface{},
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

	// Encode input context cho workflow agent.
	inputBytes, err := json.Marshal(map[string]any{
		"user_id":         userID,
		"flow":            flow,
		"available_foods": availableFoods,
	})
	if err != nil {
		return nil, fmt.Errorf("marshal workflow input: %w", err)
	}

	prompt := &genai.Content{
		Role:  "user",
		Parts: []*genai.Part{{Text: string(inputBytes)}},
	}

	for _, runErr := range r.Run(ctx, userID, sessionID, prompt, agent.RunConfig{}) {
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
func (a *NutritionAgent) runInitWorkflow(ctx context.Context, userID string, availableFoods interface{}) (*GeneratedMealPlan, error) {
	return a.runWorkflow(ctx, a.dailyWorkflowAgent, userID, FlowDaily, availableFoods)
}

// runPostWorkoutWorkflow là helper cho luồng bù Calo sau buổi tập.
func (a *NutritionAgent) runPostWorkoutWorkflow(ctx context.Context, userID string, availableFoods interface{}) (*GeneratedMealPlan, error) {
	return a.runWorkflow(ctx, a.postWorkoutWorkflowAgent, userID, FlowPostWorkout, availableFoods)
}

// runPantryWorkflow là helper cho luồng chế biến từ tủ lạnh.
func (a *NutritionAgent) runPantryWorkflow(ctx context.Context, userID string, availableFoods interface{}) (*GeneratedMealPlan, error) {
	return a.runWorkflow(ctx, a.pantryWorkflowAgent, userID, FlowPantry, availableFoods)
}

// runAdhocWorkflow là helper cho luồng gợi ý bữa nhanh.
func (a *NutritionAgent) runAdhocWorkflow(ctx context.Context, userID string, availableFoods interface{}) (*GeneratedMealPlan, error) {
	return a.runWorkflow(ctx, a.adhocWorkflowAgent, userID, FlowAdhoc, availableFoods)
}
