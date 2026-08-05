package adk

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"sync"

	"github.com/google/uuid"
	"github.com/viethung213/gym-companion/internal/nutrition/domain/aggregate"
	"github.com/viethung213/gym-companion/internal/nutrition/domain/repository"
	"github.com/viethung213/gym-companion/internal/nutrition/domain/vo"
	"google.golang.org/adk/v2/agent"
	"google.golang.org/adk/v2/agent/llmagent"
	"google.golang.org/adk/v2/model"
	"google.golang.org/adk/v2/runner"
	"google.golang.org/adk/v2/workflow"
	"google.golang.org/genai"
)

const (
	// FlowDaily là luồng sinh thực đơn 4 bữa lúc 5:00 AM.
	FlowDaily = "DAILY_4_MEALS"
	// FlowPostWorkout là luồng bù Calo sau buổi tập.
	FlowPostWorkout = "POST_WORKOUT_RECALIBRATION"
	// FlowPantry là luồng chế biến từ tủ lạnh.
	FlowPantry = "PANTRY_RECIPE"
	// FlowAdhoc là luồng gợi ý bữa ăn nhanh.
	FlowAdhoc = "ADHOC_SUGGESTION"
)

// compile-time interface check.
var _ repository.AIService = (*NutritionAgent)(nil)

// NutritionAgent orchestrates Google ADK multi-agent workflows cho module Nutrition.
type NutritionAgent struct {
	generatorNode *workflow.AgentNode

	dailyWorkflowAgent       agent.Agent
	postWorkoutWorkflowAgent agent.Agent
	pantryWorkflowAgent      agent.Agent
	adhocWorkflowAgent       agent.Agent

	// estimatorAgent dùng cho EstimateNutrient (single-turn runner call).
	estimatorAgent agent.Agent
	// insightAgent dùng cho GenerateNutritionInsight (single-turn runner call).
	insightAgent agent.Agent

	foodRepo repository.FoodItemRepository
	tools    *NutritionTools

	mu      sync.Mutex
	results map[string]*GeneratedMealPlan
}

// NewNutritionAgent khởi tạo tất cả ADK agents, tools, LLM nodes và workflow agents.
func NewNutritionAgent(
	ctx context.Context,
	_ string,
	foodRepo repository.FoodItemRepository,
) (*NutritionAgent, error) {
	if foodRepo == nil {
		return nil, errors.New("food repository is required")
	}

	a := &NutritionAgent{
		foodRepo: foodRepo,
		tools:    NewNutritionTools(foodRepo),
		results:  make(map[string]*GeneratedMealPlan),
	}

	if err := a.build(ctx); err != nil {
		return nil, err
	}

	return a, nil
}

// EstimateNutrient implements repository.AIService.
// Gọi LLM thực để ước tính dinh dưỡng từ tên món và khẩu phần.
func (a *NutritionAgent) EstimateNutrient(
	ctx context.Context,
	mealName, portion string,
) (*repository.EstimatedNutrientResult, error) {
	if a.estimatorAgent == nil {
		return nil, errors.New("estimator agent not initialised")
	}

	userMsg := fmt.Sprintf(
		"Ước tính dinh dưỡng cho món: %q, khẩu phần: %q. Trả về JSON theo định dạng đã quy định.",
		mealName, portion,
	)

	rawJSON, err := runSingleTurnLLM(ctx, a.estimatorAgent, userMsg)
	if err != nil {
		return nil, fmt.Errorf("estimate nutrient llm: %w", err)
	}

	var result repository.EstimatedNutrientResult
	//nolint:musttag
	if err := json.Unmarshal([]byte(rawJSON), &result); err != nil {
		return nil, fmt.Errorf("estimate nutrient parse json: %w", err)
	}

	return &result, nil
}

// GenerateNutritionInsight implements repository.AIService.
// Gọi LLM để phân tích lịch sử dinh dưỡng và sinh insight cải tiến.
func (a *NutritionAgent) GenerateNutritionInsight(
	ctx context.Context,
	promptCtx repository.InsightPromptContext,
) (*repository.NutritionInsightResult, error) {
	if a.insightAgent == nil {
		return nil, errors.New("insight agent not initialised")
	}

	//nolint:musttag
	historyJSON, err := json.Marshal(promptCtx)
	if err != nil {
		return nil, fmt.Errorf("generate nutrition insight marshal context: %w", err)
	}

	userMsg := "Phân tích dữ liệu dinh dưỡng sau và trả về JSON insight:\n" + string(historyJSON)

	rawJSON, err := runSingleTurnLLM(ctx, a.insightAgent, userMsg)
	if err != nil {
		return nil, fmt.Errorf("generate nutrition insight llm: %w", err)
	}

	var insight insightLLMResponse
	if err := json.Unmarshal([]byte(rawJSON), &insight); err != nil {
		return nil, fmt.Errorf("generate nutrition insight parse json: %w", err)
	}

	return mapInsightToResult(insight), nil
}

// insightLLMResponse là schema JSON mà LLM trả về cho insight.
type insightLLMResponse struct {
	OverallScore     int      `json:"overall_score"`
	Summary          string   `json:"summary"`
	Strengths        []string `json:"strengths"`
	ImprovementAreas []struct {
		Area       string  `json:"area"`
		CurrentAvg float64 `json:"current_avg"`
		Target     float64 `json:"target"`
		Suggestion string  `json:"suggestion"`
		Priority   string  `json:"priority"`
	} `json:"improvement_areas"`
	WeeklyTrend            string `json:"weekly_trend"`
	RecommendedAdjustments struct {
		CaloriesDelta     float64  `json:"calories_delta"`
		ProteinRatioDelta float64  `json:"protein_ratio_delta"`
		FocusFoods        []string `json:"focus_foods"`
	} `json:"recommended_adjustments"`
}

// mapInsightToResult chuyển đổi insightLLMResponse sang repository.NutritionInsightResult.
func mapInsightToResult(r insightLLMResponse) *repository.NutritionInsightResult {
	areas := make([]repository.ImprovementArea, 0, len(r.ImprovementAreas))
	for _, a := range r.ImprovementAreas {
		areas = append(areas, repository.ImprovementArea{
			Area:       a.Area,
			CurrentAvg: a.CurrentAvg,
			Target:     a.Target,
			Suggestion: a.Suggestion,
			Priority:   a.Priority,
		})
	}

	focusFoods := make([]string, len(r.RecommendedAdjustments.FocusFoods))
	copy(focusFoods, r.RecommendedAdjustments.FocusFoods)

	strengths := make([]string, len(r.Strengths))
	copy(strengths, r.Strengths)

	return &repository.NutritionInsightResult{
		OverallScore:     r.OverallScore,
		Summary:          r.Summary,
		Strengths:        strengths,
		ImprovementAreas: areas,
		WeeklyTrend:      r.WeeklyTrend,
		RecommendedAdjustments: repository.RecommendedAdjustments{
			CaloriesDelta:     r.RecommendedAdjustments.CaloriesDelta,
			ProteinRatioDelta: r.RecommendedAdjustments.ProteinRatioDelta,
			FocusFoods:        focusFoods,
		},
	}
}

// SelectCreativeMealOptions implements repository.AIService.
// Nhận AIMenuPromptContext từ domain, map sang internal NutritionPromptContext,
// rồi chạy workflow ADK tương ứng theo MealType.
func (a *NutritionAgent) SelectCreativeMealOptions(
	ctx context.Context,
	promptCtx repository.AIMenuPromptContext,
	lockoutRegistry vo.LockoutRegistry,
) ([]repository.GeneratedRecipeResult, error) {
	internal := NutritionPromptContext{
		UserID:               promptCtx.UserID,
		PlanDate:             promptCtx.PlanDate,
		MealType:             promptCtx.MealType,
		TargetMealCalories:   promptCtx.TargetMealCalories,
		UserRestrictions:     promptCtx.UserRestrictions,
		AvailableIngredients: promptCtx.AvailableIngredients,
	}

	var (
		wfAgent agent.Agent
		flow    string
	)

	switch internal.MealType {
	case FlowPostWorkout:
		wfAgent = a.postWorkoutWorkflowAgent
		flow = FlowPostWorkout
	case FlowPantry:
		wfAgent = a.pantryWorkflowAgent
		flow = FlowPantry
	case FlowAdhoc:
		wfAgent = a.adhocWorkflowAgent
		flow = FlowAdhoc
	default:
		wfAgent = a.dailyWorkflowAgent
		flow = FlowDaily
	}

	validator := newPlanValidator(a.foodRepo, lockoutRegistry)
	availableFoods := lockoutRegistry.FilterAvailableIngredients(internal.AvailableIngredients, internal.PlanDate)

	attemptFn := func(_ int, _ []string) (*GeneratedMealPlan, error) {
		plan, err := a.runWorkflow(ctx, wfAgent, internal.UserID, flow, availableFoods)
		if err != nil {
			return nil, err
		}
		return plan, nil
	}

	result, err := runWithRetries(ctx, validator, internal.UserRestrictions, attemptFn)
	if err != nil {
		return nil, fmt.Errorf("adk nutrition agent select options: %w", err)
	}

	return a.persistNewFoodItemsAndMap(ctx, result.Plan)
}

// persistNewFoodItemsAndMap lưu nguyên liệu mới do AI đề xuất vào CSDL, sau đó map sang GeneratedRecipeResult.
func (a *NutritionAgent) persistNewFoodItemsAndMap(
	ctx context.Context,
	plan *GeneratedMealPlan,
) ([]repository.GeneratedRecipeResult, error) {
	newCatalogNutrients := make([]vo.FoodNutrient, 0, len(plan.NewFoodCatalogItems))
	for _, newItem := range plan.NewFoodCatalogItems {
		existing, _ := a.foodRepo.FindByName(ctx, newItem.Name)
		if existing == nil {
			foodID := uuid.New().String()
			domainItem := aggregate.NewFoodItem(
				foodID,
				newItem.Name,
				newItem.Category,
				newItem.CaloriesPer100g,
				newItem.ProteinPer100g,
				newItem.CarbsPer100g,
				newItem.FatPer100g,
				newItem.AllergenTags,
				"", "", false,
			)
			if saveErr := a.foodRepo.Save(ctx, domainItem); saveErr != nil {
				log.Printf("nutrition adk: failed to save novel food item %q: %v", newItem.Name, saveErr)
			}
			newCatalogNutrients = append(newCatalogNutrients, vo.NewFoodNutrient(
				foodID, newItem.Name, newItem.Category,
				newItem.CaloriesPer100g, newItem.ProteinPer100g, newItem.CarbsPer100g, newItem.FatPer100g,
				newItem.AllergenTags, "", "", false,
			))
		}
	}

	recipeResults := make([]repository.GeneratedRecipeResult, 0, len(plan.Options))
	for _, opt := range plan.Options {
		suppIngredients := make([]aggregate.IngredientGram, 0, len(opt.SupplementaryIngredients))
		for _, supp := range opt.SupplementaryIngredients {
			suppIngredients = append(suppIngredients, aggregate.NewIngredientGram(supp.Name, supp.AmountGram, supp.IsNutiFoodProduct))
		}

		recipeResults = append(recipeResults, repository.GeneratedRecipeResult{
			RecipeName:   opt.RecipeName,
			CookingSteps: opt.CookingSteps,
			SupplementaryIngredients: append([]aggregate.IngredientGram{
				aggregate.NewIngredientGram(opt.ProteinFoodName, 150.0, false),
				aggregate.NewIngredientGram(opt.CarbFoodName, 200.0, false),
				aggregate.NewIngredientGram(opt.VeggieFoodName, 120.0, false),
			}, suppIngredients...),
			NewFoodCatalogItems: newCatalogNutrients,
			TotalProteinGrams:   opt.TotalProteinGrams,
			TotalCarbGrams:      opt.TotalCarbGrams,
			TotalFatGrams:       opt.TotalFatGrams,
		})
	}

	return recipeResults, nil
}

// putResult lưu kết quả workflow vào map nội bộ (thread-safe).
func (a *NutritionAgent) putResult(sessionID string, plan *GeneratedMealPlan) {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.results[sessionID] = plan
}

// getResult lấy kết quả workflow từ map nội bộ (thread-safe).
func (a *NutritionAgent) getResult(sessionID string) (*GeneratedMealPlan, bool) {
	a.mu.Lock()
	defer a.mu.Unlock()
	plan, ok := a.results[sessionID]
	return plan, ok
}

// buildSingleTurnAgent tạo một llm Agent standalone từ prompt file.
// Dùng với runSingleTurnLLM cho EstimateNutrient và GenerateNutritionInsight.
func buildSingleTurnAgent(promptFile string, llmModel model.LLM, name, description string) (agent.Agent, error) {
	instruction, err := readPromptFile(promptFile)
	if err != nil {
		return nil, fmt.Errorf("read prompt %s: %w", promptFile, err)
	}

	a, err := llmagent.New(llmagent.Config{
		Name:        name,
		Description: description,
		Model:       llmModel,
		Instruction: string(instruction),
	})
	if err != nil {
		return nil, fmt.Errorf("new llm agent %s: %w", name, err)
	}

	return a, nil
}

// runSingleTurnLLM chạy một LLM agent độc lập với 1 user message và trả về text response.
// Sử dụng runner.NewInMemory — đúng ADK v2 API cho standalone calls ngoài workflow context.
func runSingleTurnLLM(ctx context.Context, llmAgent agent.Agent, userMsg string) (string, error) {
	r, err := runner.NewInMemory("nutrition-app", llmAgent)
	if err != nil {
		return "", fmt.Errorf("single turn runner init: %w", err)
	}

	userID := uuid.NewString()
	sessionID := uuid.NewString()
	prompt := &genai.Content{
		Role:  "user",
		Parts: []*genai.Part{{Text: userMsg}},
	}

	var lastText string
	for event, runErr := range r.Run(ctx, userID, sessionID, prompt, agent.RunConfig{}) {
		if runErr != nil {
			return "", fmt.Errorf("single turn runner step: %w", runErr)
		}
		if event == nil || event.Content == nil {
			continue
		}
		for _, part := range event.Content.Parts {
			if part != nil && part.Text != "" {
				lastText = part.Text
			}
		}
	}

	if lastText == "" {
		return "", errors.New("single turn llm returned empty response")
	}
	return lastText, nil
}
