package adk

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"sort"
	"strings"
	"sync"

	"github.com/google/uuid"
	"github.com/viethung213/gym-companion/internal/nutrition/domain/aggregate"
	"github.com/viethung213/gym-companion/internal/nutrition/domain/repository"
	"github.com/viethung213/gym-companion/internal/nutrition/domain/service"
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
	llmModel       model.LLM
	generatorNode  *workflow.AgentNode
	generatorAgent agent.Agent

	dailyWorkflowAgent       agent.Agent
	postWorkoutWorkflowAgent agent.Agent
	pantryWorkflowAgent      agent.Agent
	adhocWorkflowAgent       agent.Agent

	// estimatorAgent dùng cho EstimateNutrient (single-turn runner call).
	estimatorAgent agent.Agent
	// insightAgent dùng cho GenerateNutritionInsight (single-turn runner call).
	insightAgent agent.Agent
	// reviewerAgent dùng để phản biện, chấm điểm và đánh giá chất lượng bữa ăn.
	reviewerAgent agent.Agent

	foodRepo repository.FoodItemRepository
	tools    *NutritionTools

	mu      sync.Mutex
	results map[string]*GeneratedMealPlan
}

// NewNutritionAgent khởi tạo tất cả ADK agents, tools, LLM nodes và workflow agents từ cấu hình môi trường.
func NewNutritionAgent(
	ctx context.Context,
	foodRepo repository.FoodItemRepository,
) (*NutritionAgent, error) {
	llmModel, err := NewFallbackLLMFromEnv(ctx)
	if err != nil {
		return nil, fmt.Errorf("new fallback llm model: %w", err)
	}

	return NewNutritionAgentWithLLM(ctx, foodRepo, llmModel)
}

// NewNutritionAgentWithLLM khởi tạo NutritionAgent với model.LLM cụ thể (hỗ trợ DI/tracking/benchmarking).
func NewNutritionAgentWithLLM(
	ctx context.Context,
	foodRepo repository.FoodItemRepository,
	llmModel model.LLM,
) (*NutritionAgent, error) {
	if foodRepo == nil {
		return nil, errors.New("food repository is required")
	}
	if llmModel == nil {
		return nil, errors.New("llm model is required")
	}

	a := &NutritionAgent{
		llmModel: llmModel,
		foodRepo: foodRepo,
		tools:    NewNutritionTools(foodRepo),
		results:  make(map[string]*GeneratedMealPlan),
	}

	if err := a.buildLLMNodes(ctx, llmModel); err != nil {
		return nil, err
	}

	if err := a.buildWorkflowAgents(ctx); err != nil {
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
	//nolint:musttag // LLM response struct does not require tags
	if err := json.Unmarshal([]byte(cleanJSONResponse(rawJSON)), &result); err != nil {
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

	//nolint:musttag // LLM response struct does not require tags
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
	if err := json.Unmarshal([]byte(cleanJSONResponse(rawJSON)), &insight); err != nil {
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
//
//nolint:gocritic // r is an internal response struct
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
//
//nolint:gocritic // hugeParam: implements repository.AIService
func (a *NutritionAgent) SelectCreativeMealOptions(
	ctx context.Context,
	promptCtx repository.AIMenuPromptContext,
	lockoutRegistry vo.LockoutRegistry,
) ([]repository.GeneratedRecipeResult, error) {
	recipes, _, err := a.SelectCreativeMealOptionsWithDetails(ctx, promptCtx, lockoutRegistry)
	return recipes, err
}

// SelectCreativeMealOptionsWithDetails thực hiện sinh thực đơn và trả về kèm kết quả PlanResult
// (bao gồm điểm số Reviewer, số vòng lặp, log đánh giá chi tiết).
//
//nolint:gocritic // hugeParam: promptCtx holds required generation inputs
func (a *NutritionAgent) SelectCreativeMealOptionsWithDetails(
	ctx context.Context,
	promptCtx repository.AIMenuPromptContext,
	lockoutRegistry vo.LockoutRegistry,
) ([]repository.GeneratedRecipeResult, *PlanResult, error) {
	filteredFoods := lockoutRegistry.FilterAvailableIngredients(promptCtx.AvailableIngredients, promptCtx.PlanDate)
	if len(filteredFoods) > 21 {
		filteredFoods = service.SelectDiverseCandidates(
			filteredFoods,
			lockoutRegistry,
			promptCtx.UserRestrictions,
			promptCtx.UserID,
			promptCtx.PlanDate,
			service.DefaultCandidateConfig(),
		)
	}
	availableDTOs := ToFoodNutrientDTOs(filteredFoods)

	internal := NutritionPromptContext{
		UserID:                   promptCtx.UserID,
		PlanDate:                 promptCtx.PlanDate,
		MealType:                 promptCtx.MealType,
		NutritionGoal:            promptCtx.NutritionGoal,
		TargetMealCalories:       promptCtx.TargetMealCalories,
		TargetDailyCalories:      promptCtx.TargetDailyCalories,
		TargetProteinGrams:       promptCtx.TargetProteinGrams,
		TargetCarbGrams:          promptCtx.TargetCarbGrams,
		TargetFatGrams:           promptCtx.TargetFatGrams,
		UserRestrictions:         promptCtx.UserRestrictions,
		MandatoryUserIngredients: ToFoodNutrientDTOs(promptCtx.MandatoryUserIngredients),
		SupplementaryIngredients: ToFoodNutrientDTOs(promptCtx.SupplementaryIngredients),
		AvailableIngredients:     availableDTOs,
		LockoutRegistry:          lockoutRegistry,
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

	attemptFn := func(round int, fb *attemptFeedback) (*GeneratedMealPlan, error) {
		if round > 1 && fb != nil && fb.PreviousPlan != nil && len(fb.PreviousPlan.Options) > 0 {
			repairedPlan, repairErr := a.repairFlawedMeals(ctx, internal, fb)
			if repairErr == nil && repairedPlan != nil && len(repairedPlan.Options) == len(fb.PreviousPlan.Options) {
				return repairedPlan, nil
			}
			log.Printf("nutrition: single-meal repair failed: %v, retaining previous plan to enforce token limit", repairErr)
			return fb.PreviousPlan, nil
		}

		plan, err := a.runWorkflow(ctx, wfAgent, internal, fb)
		if err != nil {
			return nil, err
		}
		return plan, nil
	}

	var reviewFn planReviewFunc
	if flow != FlowAdhoc && flow != FlowPostWorkout {
		reviewFn = a.reviewMealPlan(ctx, internal)
	}

	result, err := runWithRetries(ctx, validator, availableDTOs, internal.UserRestrictions, attemptFn, reviewFn)
	if err != nil {
		return nil, nil, fmt.Errorf("adk nutrition agent select options: %w", err)
	}

	recipes, err := a.persistNewFoodItemsAndMap(ctx, result.Plan, availableDTOs)
	return recipes, result, err
}

// reviewMealPlan tạo hàm chấm điểm và phản biện thực đơn qua reviewerAgent.
//
//nolint:gocritic // hugeParam: pCtx holds prompt context
func (a *NutritionAgent) reviewMealPlan(
	ctx context.Context,
	pCtx NutritionPromptContext,
) planReviewFunc {
	return func(round int, plan *GeneratedMealPlan, report ValidationReport, prior []ReviewNote) (*MealPlanReview, error) {
		if a.reviewerAgent == nil {
			return nil, errors.New("no reviewer configured")
		}

		req := ReviewRequest{
			OriginalTask:        pCtx.MealType,
			NutritionGoal:       pCtx.NutritionGoal,
			TargetDailyCalories: pCtx.TargetDailyCalories,
			TargetProteinGrams:  pCtx.TargetProteinGrams,
			TargetCarbGrams:     pCtx.TargetCarbGrams,
			TargetFatGrams:      pCtx.TargetFatGrams,
			UserRestrictions:    pCtx.UserRestrictions,
			GeneratorOutput:     plan.ToCompactForReview(),
			ValidationResult:    report,
			ReviewRound:         round,
			PreviousFeedback:    prior,
		}

		rawBytes, err := json.Marshal(req)
		if err != nil {
			return nil, fmt.Errorf("marshal review request: %w", err)
		}

		rawJSON, err := runSingleTurnLLM(ctx, a.reviewerAgent, string(rawBytes))
		if err != nil {
			return nil, fmt.Errorf("reviewer llm call: %w", err)
		}

		var verdict MealPlanReview
		if err := json.Unmarshal([]byte(cleanJSONResponse(rawJSON)), &verdict); err != nil {
			return nil, fmt.Errorf("parse review json: %w", err)
		}

		if err := validateReview(&verdict, prior); err != nil {
			return nil, err
		}

		return &verdict, nil
	}
}

// persistNewFoodItemsAndMap maps GeneratedMealPlan to GeneratedRecipeResult without inserting novel items into the database.
func (a *NutritionAgent) persistNewFoodItemsAndMap(
	_ context.Context,
	plan *GeneratedMealPlan,
	_ []FoodNutrientDTO,
) ([]repository.GeneratedRecipeResult, error) {
	recipeResults := make([]repository.GeneratedRecipeResult, 0, len(plan.Options))
	for i := range plan.Options {
		opt := &plan.Options[i]
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
			NewFoodCatalogItems: nil,
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

// repairFlawedMeals thay thế cục bộ (single-meal replacement) cho các món bị đánh lỗi ở Vòng 1.
// Giữ nguyên 100% các món đã đạt, chỉ yêu cầu AI sinh lại đúng (các) món bị lỗi
// bằng danh sách nguyên liệu chưa dùng, giúp tiết kiệm triệt để token và thời gian phản hồi.
//
//nolint:gocritic // hugeParam: pCtx holds prompt context
func (a *NutritionAgent) repairFlawedMeals(
	ctx context.Context,
	pCtx NutritionPromptContext,
	fb *attemptFeedback,
) (*GeneratedMealPlan, error) {
	if fb == nil || fb.PreviousPlan == nil || len(fb.PreviousPlan.Options) == 0 {
		return nil, errors.New("no previous plan to repair")
	}

	flawedMap := make(map[int]ReviewNote)

	// 1. Quét feedback từ Reviewer Agent (có option_index, tên món, hoặc từ khóa nguyên liệu)
	if fb.Review != nil {
		for _, note := range fb.Review.Feedback {
			matched := false
			if note.OptionIndex >= 1 && note.OptionIndex <= len(fb.PreviousPlan.Options) {
				flawedMap[note.OptionIndex-1] = note
				matched = true
			} else if note.RecipeName != "" {
				for i := range fb.PreviousPlan.Options {
					if strings.EqualFold(fb.PreviousPlan.Options[i].RecipeName, note.RecipeName) {
						flawedMap[i] = note
						matched = true
						break
					}
				}
			}

			// 1.b: Tìm theo số thứ tự nhắc trong Detail/Fix (ví dụ: "option 2", "bữa sáng")
			if !matched {
				textLower := strings.ToLower(note.Detail + " " + note.Fix)
				for i := range fb.PreviousPlan.Options {
					optMarker := fmt.Sprintf("option %d", i+1)
					if strings.Contains(textLower, optMarker) {
						flawedMap[i] = note
						matched = true
						break
					}
				}
			}

			// 1.c: Tìm theo nguyên liệu được nêu trong phản biện
			if !matched {
				textLower := strings.ToLower(note.Detail + " " + note.Fix)
				bestScore := 0
				bestIdx := -1
				for i := range fb.PreviousPlan.Options {
					opt := &fb.PreviousPlan.Options[i]
					score := 0
					if opt.ProteinFoodName != "" && strings.Contains(textLower, strings.ToLower(opt.ProteinFoodName)) {
						score += 2
					}
					if opt.CarbFoodName != "" && strings.Contains(textLower, strings.ToLower(opt.CarbFoodName)) {
						score += 2
					}
					if opt.VeggieFoodName != "" && strings.Contains(textLower, strings.ToLower(opt.VeggieFoodName)) {
						score += 2
					}
					if score > bestScore {
						bestScore = score
						bestIdx = i
					}
				}
				if bestIdx != -1 && bestScore >= 2 {
					flawedMap[bestIdx] = note
					matched = true
				}
			}
		}
	}

	// 2. Quét issues từ Gate 1 (deterministic validator, ví dụ: "option 2: ...")
	for _, issue := range fb.Issues {
		var optNum int
		if n, _ := fmt.Sscanf(issue, "option %d:", &optNum); n == 1 &&
			optNum >= 1 && optNum <= len(fb.PreviousPlan.Options) {
			idx := optNum - 1
			if _, exists := flawedMap[idx]; !exists {
				flawedMap[idx] = ReviewNote{
					OptionIndex: optNum,
					Area:        "validator_issue",
					Detail:      issue,
					Fix:         "Replace invalid or conflicting ingredient with active catalog items",
				}
			}
		}
	}

	// Nếu chưa cô lập được món cụ thể nhưng có phản biện từ Reviewer, chọn option đầu tiên để sửa
	if len(flawedMap) == 0 && fb.Review != nil && len(fb.Review.Feedback) > 0 {
		flawedMap[0] = fb.Review.Feedback[0]
	}

	// Nếu có nhiều hơn 2 món bị chê, chỉ chọn tối đa 2 món đầu tiên để sửa đơn món,
	// tuyệt đối không sinh lại toàn bộ 8 món để bảo toàn giới hạn 20k tokens.
	if len(flawedMap) > 2 {
		prunedMap := make(map[int]ReviewNote, 2)
		count := 0
		for idx, note := range flawedMap {
			prunedMap[idx] = note
			count++
			if count >= 2 {
				break
			}
		}
		flawedMap = prunedMap
	}

	if len(flawedMap) == 0 {
		return nil, errors.New("no flawed options detected for single-meal replacement")
	}

	newPlan := clonePlan(fb.PreviousPlan)

	// 3. Thu thập các nguyên liệu ĐÃ ĐƯỢC DÙNG bởi các món ĐẠT (không bị lỗi)
	usedIDs := make(map[string]bool)
	for i := range newPlan.Options {
		if _, isFlawed := flawedMap[i]; !isFlawed {
			opt := &newPlan.Options[i]
			if opt.ProteinFoodID != "" {
				usedIDs[opt.ProteinFoodID] = true
			}
			if opt.CarbFoodID != "" {
				usedIDs[opt.CarbFoodID] = true
			}
			if opt.VeggieFoodID != "" {
				usedIDs[opt.VeggieFoodID] = true
			}
		}
	}

	// 4. Lọc danh sách nguyên liệu còn lại chưa dùng ở đợt 1
	remaining := make([]FoodNutrientDTO, 0, len(pCtx.AvailableIngredients))
	for _, ing := range pCtx.AvailableIngredients {
		if !usedIDs[ing.ID] {
			remaining = append(remaining, ing)
		}
	}
	if len(remaining) < 6 {
		remaining = pCtx.AvailableIngredients // Tránh cạn kiệt nguyên liệu cho món sửa
	}

	// 5. Gom tất cả các món lỗi vào danh sách để gửi batch trong 1 request duy nhất
	targets := make([]flawedTarget, 0, len(flawedMap))
	for idx, note := range flawedMap {
		slotName := "Meal Option"
		switch idx / 2 {
		case 0:
			slotName = fmt.Sprintf("BREAKFAST (Option %d)", (idx%2)+1)
		case 1:
			slotName = fmt.Sprintf("LUNCH (Option %d)", (idx%2)+1)
		case 2:
			slotName = fmt.Sprintf("DINNER (Option %d)", (idx%2)+1)
		case 3:
			slotName = fmt.Sprintf("SNACK (Option %d)", (idx%2)+1)
		}
		targets = append(targets, flawedTarget{
			optIdx:   idx,
			oldOpt:   newPlan.Options[idx],
			note:     note,
			slotName: slotName,
		})
	}

	sort.Slice(targets, func(i, j int) bool {
		return targets[i].optIdx < targets[j].optIdx
	})

	// 6. Rút gọn danh sách nguyên liệu (tối đa 6 món/nhóm) để tránh phình to prompt
	compactRemaining := selectCompactIngredients(remaining, 6)

	// 7. GỌI DUY NHẤT 1 REQUEST SỬA TẤT CẢ CÁC MÓN LỖI CÙNG LÚC
	replacements, err := a.generateBatchReplacementOptions(ctx, pCtx, targets, compactRemaining)
	if err != nil {
		return nil, fmt.Errorf("batch replacement failed: %w", err)
	}

	for i := range targets {
		if i >= len(replacements) {
			continue
		}
		t := &targets[i]
		rep := replacements[i]
		if rep.ProteinFoodID == "" {
			rep.ProteinFoodID = t.oldOpt.ProteinFoodID
			rep.ProteinFoodName = t.oldOpt.ProteinFoodName
		}
		if rep.CarbFoodID == "" {
			rep.CarbFoodID = t.oldOpt.CarbFoodID
			rep.CarbFoodName = t.oldOpt.CarbFoodName
		}
		if rep.VeggieFoodID == "" {
			rep.VeggieFoodID = t.oldOpt.VeggieFoodID
			rep.VeggieFoodName = t.oldOpt.VeggieFoodName
		}
		if len(rep.CookingSteps) == 0 {
			rep.CookingSteps = t.oldOpt.CookingSteps
		}
		if rep.RecipeName == "" {
			rep.RecipeName = t.oldOpt.RecipeName
		}
		newPlan.Options[t.optIdx] = rep
	}

	return newPlan, nil
}

// selectCompactIngredients chọn lọc tối đa maxPerCategory món cho mỗi nhóm để giảm thiểu input tokens.
func selectCompactIngredients(all []FoodNutrientDTO, maxPerCategory int) []FoodNutrientDTO {
	byCat := make(map[string][]FoodNutrientDTO)
	for i := range all {
		cat := strings.ToUpper(strings.TrimSpace(all[i].Category))
		byCat[cat] = append(byCat[cat], all[i])
	}

	res := make([]FoodNutrientDTO, 0, maxPerCategory*4)
	for _, cat := range []string{"PROTEIN", "CARB", "VEGGIE", "SNACK"} {
		items := byCat[cat]
		if len(items) > maxPerCategory {
			items = items[:maxPerCategory]
		}
		res = append(res, items...)
	}
	if len(res) == 0 {
		return all
	}
	return res
}

// flawedTarget định nghĩa thông tin món ăn cần sửa.
type flawedTarget struct {
	optIdx   int
	oldOpt   GeneratedMealOption
	note     ReviewNote
	slotName string
}

// generateBatchReplacementOptions gọi Generator để sinh đồng thời các món ăn thay thế trong DUY NHẤT 1 request.
//
//nolint:gocritic // hugeParam: pCtx holds prompt context
func (a *NutritionAgent) generateBatchReplacementOptions(
	ctx context.Context,
	pCtx NutritionPromptContext,
	targets []flawedTarget,
	compactIngredients []FoodNutrientDTO,
) ([]GeneratedMealOption, error) {
	if a.llmModel == nil {
		return nil, errors.New("llm model not initialised")
	}

	itemsToReplace := make([]map[string]any, 0, len(targets))
	for i := range targets {
		t := &targets[i]
		itemsToReplace = append(itemsToReplace, map[string]any{
			"option_slot":          t.slotName,
			"target_meal_calories": pCtx.TargetDailyCalories * 0.3,
			"flawed_previous_meal": map[string]any{
				"recipe_name": t.oldOpt.RecipeName,
				"protein":     t.oldOpt.ProteinFoodName,
				"carb":        t.oldOpt.CarbFoodName,
				"veggie":      t.oldOpt.VeggieFoodName,
			},
			"reviewer_defect_detail": t.note.Detail,
			"reviewer_fix_guidance":  t.note.Fix,
		})
	}

	reqMap := map[string]any{
		"task":                  "REPLACE_FLAWED_MEALS_BATCH",
		"nutrition_goal":        pCtx.NutritionGoal,
		"user_restrictions":     pCtx.UserRestrictions,
		"items_to_replace":      itemsToReplace,
		"available_ingredients": compactIngredients,
	}

	rawBytes, err := json.Marshal(reqMap)
	if err != nil {
		return nil, fmt.Errorf("marshal batch replacement request: %w", err)
	}

	instruction := fmt.Sprintf(`You are the GymCompanion AI Nutritionist.
Your task is to generate EXACTLY %d replacement meal option(s) in pure JSON, one for each item in items_to_replace, in the EXACT SAME ORDER.
Output MUST be pure JSON with NO markdown wrappers and strictly follow this schema:
{
  "options": [
    {
      "protein_food_id": "<UUID from available_ingredients>",
      "protein_food_name": "<name of protein>",
      "carb_food_id": "<UUID from available_ingredients>",
      "carb_food_name": "<name of carb>",
      "veggie_food_id": "<UUID from available_ingredients>",
      "veggie_food_name": "<name of veggie>",
      "cooking_style": "Steamed / Grilled / Pan-seared / Boiled / Stir-fried",
      "recipe_name": "<Short English name < 5 words>",
      "cooking_steps": [
        "Step 1 practical instruction <= 15 words.",
        "Step 2 practical instruction <= 15 words.",
        "Step 3 practical instruction <= 15 words."
      ],
      "supplementary_ingredients": [
        {"name": "Olive oil", "amount_gram": 5.0, "is_nutifood_product": false}
      ],
      "total_protein_grams": 35.0,
      "total_carb_grams": 40.0,
      "total_fat_grams": 10.0
    }
  ]
}
CRITICAL RULES:
1. Return EXACTLY %d meal option(s) in the options array.
2. Every single field in each option MUST be filled. NEVER leave protein_food_id, carb_food_id, veggie_food_id, cooking_steps or recipe_name empty or null.
3. Use ONLY exact IDs from available_ingredients.
4. recipe_name must be under 5 words in English.
5. cooking_steps must have 3-5 concise steps, each step <= 15 words.
6. Fix each reviewer defect and follow reviewer_fix_guidance.
`, len(targets), len(targets))

	req := &model.LLMRequest{
		Contents: []*genai.Content{
			{
				Role: "user",
				Parts: []*genai.Part{
					{Text: instruction + "\n" + string(rawBytes)},
				},
			},
		},
		Config: &genai.GenerateContentConfig{
			ResponseMIMEType: "application/json",
			MaxOutputTokens:  2000,
		},
	}

	var rawJSON string
	for resp, genErr := range a.llmModel.GenerateContent(ctx, req, false) {
		if genErr != nil {
			return nil, fmt.Errorf("batch replacement llm call: %w", genErr)
		}
		if resp != nil && resp.Content != nil {
			for _, part := range resp.Content.Parts {
				if part != nil && part.Text != "" {
					rawJSON = part.Text
				}
			}
		}
	}
	if rawJSON == "" {
		return nil, errors.New("batch replacement llm returned empty response")
	}

	plan, err := parseMealPlanFromNode(rawJSON)
	if err != nil {
		return nil, fmt.Errorf("parse batch replacement json: %w", err)
	}
	if len(plan.Options) == 0 {
		return nil, errors.New("batch replacement produced empty options")
	}

	return plan.Options, nil
}
