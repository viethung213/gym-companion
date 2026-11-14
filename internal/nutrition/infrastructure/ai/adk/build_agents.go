package adk

import (
	"context"
	"embed"
	"encoding/json"
	"errors"
	"fmt"
	"iter"
	"log/slog"
	"path/filepath"
	"strings"

	"github.com/viethung213/gym-companion/internal/nutrition/infrastructure/config"
	"google.golang.org/adk/v2/agent"
	"google.golang.org/adk/v2/agent/llmagent"
	"google.golang.org/adk/v2/model"
	"google.golang.org/adk/v2/session"
	"google.golang.org/adk/v2/tool"
	"google.golang.org/adk/v2/tool/functiontool"
	"google.golang.org/adk/v2/workflow"
	"google.golang.org/genai"
)

//go:embed prompts/*.txt
var promptFS embed.FS

// readPromptFile đọc file prompt từ embedded filesystem promptFS.
func readPromptFile(path string) ([]byte, error) {
	filename := filepath.Base(path)
	return promptFS.ReadFile("prompts/" + filename)
}

// build khởi tạo LLM model (với cơ chế Fallback tự động), LLM generator node và tất cả workflow agents.
func (a *NutritionAgent) build(ctx context.Context) error {
	llmModel, err := NewFallbackLLMFromEnv(ctx)
	if err != nil {
		return fmt.Errorf("new fallback llm model: %w", err)
	}

	if err := a.buildLLMNodes(ctx, llmModel); err != nil {
		return err
	}

	return a.buildWorkflowAgents(ctx)
}

// buildLLMNodes đọc prompt generator.txt và khởi tạo LLM generator node.
// Cũng khởi tạo estimatorNode và insightNode cho các single-turn call.
func (a *NutritionAgent) buildLLMNodes(_ context.Context, llmModel model.LLM) error {
	generatorInstruction, err := readPromptFile("prompts/generator.txt")
	if err != nil {
		return fmt.Errorf("read generator prompt: %w", err)
	}

	nutritionTools, err := a.buildADKTools()
	if err != nil {
		return fmt.Errorf("build adk tools: %w", err)
	}

	nutritionCfg := config.LoadConfig()
	maxTokens := int32(nutritionCfg.MaxOutputTokens)
	if maxTokens <= 0 {
		maxTokens = 8192
	}

	zero := int32(0)
	generatorAgent, err := llmagent.New(llmagent.Config{
		Name:        "NutritionGeneratorAgent",
		Description: "Chuyên gia dinh dưỡng AI: chọn thực phẩm sáng tạo, cân bằng Macro theo TDEE.",
		Model:       llmModel,
		Instruction: string(generatorInstruction),
		Tools:       nutritionTools,
		OutputKey:   "generated_meal_plan_json",
		GenerateContentConfig: &genai.GenerateContentConfig{
			MaxOutputTokens: maxTokens,
			ThinkingConfig: &genai.ThinkingConfig{
				ThinkingBudget: &zero,
			},
		},
	})
	if err != nil {
		return fmt.Errorf("new generator agent: %w", err)
	}

	generatorNode, err := workflow.NewAgentNode(generatorAgent, workflow.NodeConfig{})
	if err != nil {
		return fmt.Errorf("new generator node: %w", err)
	}

	a.generatorNode = generatorNode
	a.generatorAgent = generatorAgent

	// Single-turn: EstimateNutrient
	estimatorAgent, err := buildSingleTurnAgent(
		"prompts/estimate_nutrient.txt",
		llmModel,
		"NutritionEstimatorAgent",
		"Ước tính dinh dưỡng (Calo/Protein/Carb/Fat) từ tên món và khẩu phần.",
	)
	if err != nil {
		return fmt.Errorf("build estimator agent: %w", err)
	}
	a.estimatorAgent = estimatorAgent

	// Single-turn: GenerateNutritionInsight
	insightAgent, err := buildSingleTurnAgent(
		"prompts/nutrition_insight.txt",
		llmModel,
		"NutritionInsightAgent",
		"Phân tích lịch sử dinh dưỡng và đề xuất hướng cải thiện cá nhân hóa.",
	)
	if err != nil {
		return fmt.Errorf("build insight agent: %w", err)
	}
	a.insightAgent = insightAgent

	// Reviewer Agent: phản biện và chấm điểm thực đơn
	reviewerInstruction, err := readPromptFile("prompts/evaluator.txt")
	if err != nil {
		return fmt.Errorf("read reviewer prompt: %w", err)
	}

	maxReviewerTokens := int32(1000)
	if maxReviewerTokens > maxTokens {
		maxReviewerTokens = maxTokens
	}

	reviewerAgent, err := llmagent.New(llmagent.Config{
		Name:         "NutritionReviewerAgent",
		Description:  "Chuyên gia ẩm thực & dinh dưỡng AI: phản biện chất lượng món ăn, hương vị, tính khả thi khi nấu.",
		Model:        llmModel,
		Instruction:  string(reviewerInstruction),
		OutputSchema: buildMealPlanReviewSchema(),
		GenerateContentConfig: &genai.GenerateContentConfig{
			MaxOutputTokens: maxReviewerTokens,
		},
	})
	if err != nil {
		return fmt.Errorf("new reviewer agent: %w", err)
	}
	a.reviewerAgent = reviewerAgent

	return nil
}

// buildMealPlanReviewSchema định nghĩa cấu trúc JSON phản biện của Reviewer.
// Cố tình không có field options/plan để đảm bảo Reviewer chỉ phản biện chứ không tự sửa bài.
func buildMealPlanReviewSchema() *genai.Schema {
	return &genai.Schema{
		Type: genai.TypeObject,
		Properties: map[string]*genai.Schema{
			"approved":   {Type: genai.TypeBoolean},
			"score":      {Type: genai.TypeInteger},
			"confidence": {Type: genai.TypeNumber},
			"feedback": {
				Type: genai.TypeArray,
				Items: &genai.Schema{
					Type: genai.TypeObject,
					Properties: map[string]*genai.Schema{
						"option_index": {Type: genai.TypeInteger},
						"recipe_name":  {Type: genai.TypeString},
						"area":         {Type: genai.TypeString},
						"detail":       {Type: genai.TypeString},
						"fix":          {Type: genai.TypeString},
					},
					Required: []string{"area", "detail", "fix"},
				},
			},
			"notes": {
				Type:  genai.TypeArray,
				Items: &genai.Schema{Type: genai.TypeString},
			},
			"previous_feedback": {
				Type: genai.TypeArray,
				Items: &genai.Schema{
					Type: genai.TypeObject,
					Properties: map[string]*genai.Schema{
						"area":     {Type: genai.TypeString},
						"applied":  {Type: genai.TypeBoolean},
						"evidence": {Type: genai.TypeString},
					},
					Required: []string{"area", "applied", "evidence"},
				},
			},
		},
		Required: []string{"approved", "score", "confidence"},
	}
}

// fetchCatalogArgs là args schema cho tool fetch_food_catalog.
type fetchCatalogArgs struct {
	Category   string   `json:"category"`
	Limit      int      `json:"limit,omitempty"`
	Page       int      `json:"page,omitempty"`
	ExcludeIDs []string `json:"exclude_ids,omitempty"`
}

// macroGramArgs là args schema cho tool calculate_macro_gram.
type macroGramArgs struct {
	TargetCalories float64 `json:"target_calories"`
	ProteinRatio   float64 `json:"protein_ratio"`
	CarbRatio      float64 `json:"carb_ratio"`
	FatRatio       float64 `json:"fat_ratio"`
}

// macroGramResult là kết quả của tool calculate_macro_gram.
type macroGramResult struct {
	ProteinGrams float64 `json:"protein_grams"`
	CarbGrams    float64 `json:"carb_grams"`
	FatGrams     float64 `json:"fat_grams"`
}

// buildADKTools tạo danh sách tool.Tool từ NutritionTools để truyền vào llmagent.
func (a *NutritionAgent) buildADKTools() ([]tool.Tool, error) {
	fetchTool, err := functiontool.New(
		functiontool.Config{
			Name: "fetch_food_catalog",
			Description: "Tải 5-8 nguyên liệu an toàn ngẫu nhiên theo nhóm (PROTEIN, CARB, VEGGIE, SNACK) " +
				"đã tự động lọc sạch dị ứng và trùng lặp chu kỳ 7 ngày. Hỗ trợ 'page' (bắt đầu từ 1, tăng dần) " +
				"và 'exclude_ids' (danh sách ID các món xung đột vị cần loại trừ).",
		},
		func(ctx agent.Context, args fetchCatalogArgs) (interface{}, error) {
			opts := FetchCatalogOptions{
				Limit:      args.Limit,
				Page:       args.Page,
				ExcludeIDs: args.ExcludeIDs,
			}
			return a.tools.FetchActiveFoodCatalog(ctx, ctx.SessionID(), args.Category, opts)
		},
	)
	if err != nil {
		return nil, fmt.Errorf("make fetch catalog tool: %w", err)
	}

	macroTool, err := functiontool.New(
		functiontool.Config{
			Name:        "calculate_macro_gram",
			Description: "Tính Gram Đạm, Tinh bột, Chất béo dựa theo Calo mục tiêu và tỷ lệ Macro (0-1).",
		},
		func(_ agent.Context, args macroGramArgs) (macroGramResult, error) {
			p, c, f := a.tools.CalculateMacroGramRatio(args.TargetCalories, args.ProteinRatio, args.CarbRatio, args.FatRatio)
			return macroGramResult{ProteinGrams: p, CarbGrams: c, FatGrams: f}, nil
		},
	)
	if err != nil {
		return nil, fmt.Errorf("make macro gram tool: %w", err)
	}

	return []tool.Tool{fetchTool, macroTool}, nil
}

// buildWorkflowAgents tạo 4 workflow agents cho 4 luồng dinh dưỡng.
func (a *NutritionAgent) buildWorkflowAgents(ctx context.Context) error {
	var err error

	a.dailyWorkflowAgent, err = a.buildDailyWorkflowAgent(ctx)
	if err != nil {
		return fmt.Errorf("build daily workflow agent: %w", err)
	}

	a.postWorkoutWorkflowAgent, err = a.buildPostWorkoutWorkflowAgent(ctx)
	if err != nil {
		return fmt.Errorf("build post-workout workflow agent: %w", err)
	}

	a.pantryWorkflowAgent, err = a.buildPantryWorkflowAgent(ctx)
	if err != nil {
		return fmt.Errorf("build pantry workflow agent: %w", err)
	}

	a.adhocWorkflowAgent, err = a.buildAdhocWorkflowAgent(ctx)
	if err != nil {
		return fmt.Errorf("build adhoc workflow agent: %w", err)
	}

	return nil
}

// cleanJSONResponse extracts the JSON payload from LLM output by locating
// the first '{' or '[' and the last '}' or ']'. This safely removes markdown backticks
// (e.g. ```json), leading text, or trailing prose emitted by LLM models.
//
//nolint:gocritic // ifElseChain is readable for index finding
func cleanJSONResponse(raw string) string {
	s := strings.TrimSpace(raw)

	startObj := strings.Index(s, "{")
	startArr := strings.Index(s, "[")
	start := -1
	if startObj != -1 && startArr != -1 {
		if startObj < startArr {
			start = startObj
		} else {
			start = startArr
		}
	} else if startObj != -1 {
		start = startObj
	} else if startArr != -1 {
		start = startArr
	}

	endObj := strings.LastIndex(s, "}")
	endArr := strings.LastIndex(s, "]")
	end := -1
	if endObj != -1 && endArr != -1 {
		if endObj > endArr {
			end = endObj
		} else {
			end = endArr
		}
	} else if endObj != -1 {
		end = endObj
	} else if endArr != -1 {
		end = endArr
	}

	if start != -1 && end != -1 && end > start {
		return s[start : end+1]
	}

	s = strings.TrimPrefix(s, "```json")
	s = strings.TrimPrefix(s, "```JSON")
	s = strings.TrimPrefix(s, "```")
	s = strings.TrimSuffix(s, "```")
	return strings.TrimSpace(s)
}

// parseMealPlanFromNode parse JSON output từ generator node thành GeneratedMealPlan.
func parseMealPlanFromNode(planJSON string) (*GeneratedMealPlan, error) {
	cleanedJSON := cleanJSONResponse(planJSON)

	var plan GeneratedMealPlan
	if err := json.Unmarshal([]byte(cleanedJSON), &plan); err != nil {
		return nil, fmt.Errorf("parse meal plan json: %w", err)
	}
	if len(plan.Options) == 0 {
		return nil, errors.New("generator returned empty options")
	}
	return &plan, nil
}

// buildDailyWorkflowAgent tạo workflow agent cho luồng sinh thực đơn 5:00 AM.
func (a *NutritionAgent) buildDailyWorkflowAgent(_ context.Context) (agent.Agent, error) {
	systemPrompt, err := readPromptFile("prompts/daily.txt")
	if err != nil {
		return nil, fmt.Errorf("read daily prompt: %w", err)
	}

	node := workflow.NewDynamicNode(
		"daily_menu_workflow",
		func(nodeCtx agent.Context, userID string, _ func(*session.Event) error) (*GeneratedMealPlan, error) {
			if setErr := nodeCtx.State().Set("system_context", string(systemPrompt)); setErr != nil {
				return nil, fmt.Errorf("set daily system context: %w", setErr)
			}

			planJSON, genErr := workflow.RunNode[string](nodeCtx, a.generatorNode, userID)
			if genErr != nil {
				return nil, fmt.Errorf("daily generator: %w", genErr)
			}

			plan, parseErr := parseMealPlanFromNode(planJSON)
			if parseErr != nil {
				return nil, parseErr
			}

			a.putResult(nodeCtx.SessionID(), plan)
			return plan, nil
		},
		workflow.NodeConfig{},
	)

	wf, err := workflow.New("daily_menu_wf", workflow.Chain(workflow.Start, node))
	if err != nil {
		return nil, fmt.Errorf("new daily workflow: %w", err)
	}

	return agent.New(agent.Config{
		Name:        "DailyMenuWorkflowAgent",
		Description: "Sinh thực đơn 4 bữa đầy đủ dinh dưỡng lúc 5:00 AM mỗi ngày.",
		Run: func(ic agent.InvocationContext) iter.Seq2[*session.Event, error] {
			return wf.Run(ic)
		},
	})
}

// buildPostWorkoutWorkflowAgent tạo workflow agent cho luồng bù Calo sau buổi tập.
func (a *NutritionAgent) buildPostWorkoutWorkflowAgent(_ context.Context) (agent.Agent, error) {
	systemPrompt, err := readPromptFile("prompts/post_workout.txt")
	if err != nil {
		return nil, fmt.Errorf("read post_workout prompt: %w", err)
	}

	node := workflow.NewDynamicNode(
		"post_workout_workflow",
		func(nodeCtx agent.Context, userID string, _ func(*session.Event) error) (*GeneratedMealPlan, error) {
			if setErr := nodeCtx.State().Set("system_context", string(systemPrompt)); setErr != nil {
				return nil, fmt.Errorf("set post_workout system context: %w", setErr)
			}

			planJSON, genErr := workflow.RunNode[string](nodeCtx, a.generatorNode, userID)
			if genErr != nil {
				return nil, fmt.Errorf("post_workout generator: %w", genErr)
			}

			plan, parseErr := parseMealPlanFromNode(planJSON)
			if parseErr != nil {
				return nil, parseErr
			}

			a.putResult(nodeCtx.SessionID(), plan)
			slog.Info("Post-workout recalibration plan generated", slog.String("session_id", nodeCtx.SessionID()))
			return plan, nil
		},
		workflow.NodeConfig{},
	)

	wf, err := workflow.New("post_workout_wf", workflow.Chain(workflow.Start, node))
	if err != nil {
		return nil, fmt.Errorf("new post_workout workflow: %w", err)
	}

	return agent.New(agent.Config{
		Name:        "PostWorkoutWorkflowAgent",
		Description: "Bù Calo và Protein phục hồi cơ bắp ngay sau khi hoàn thành buổi tập.",
		Run: func(ic agent.InvocationContext) iter.Seq2[*session.Event, error] {
			return wf.Run(ic)
		},
	})
}

// buildPantryWorkflowAgent tạo workflow agent cho luồng chế biến từ tủ lạnh.
func (a *NutritionAgent) buildPantryWorkflowAgent(_ context.Context) (agent.Agent, error) {
	systemPrompt, err := readPromptFile("prompts/pantry.txt")
	if err != nil {
		return nil, fmt.Errorf("read pantry prompt: %w", err)
	}

	node := workflow.NewDynamicNode(
		"pantry_recipe_workflow",
		func(nodeCtx agent.Context, userID string, _ func(*session.Event) error) (*GeneratedMealPlan, error) {
			if setErr := nodeCtx.State().Set("system_context", string(systemPrompt)); setErr != nil {
				return nil, fmt.Errorf("set pantry system context: %w", setErr)
			}

			planJSON, genErr := workflow.RunNode[string](nodeCtx, a.generatorNode, userID)
			if genErr != nil {
				return nil, fmt.Errorf("pantry generator: %w", genErr)
			}

			plan, parseErr := parseMealPlanFromNode(planJSON)
			if parseErr != nil {
				return nil, parseErr
			}

			a.putResult(nodeCtx.SessionID(), plan)
			return plan, nil
		},
		workflow.NodeConfig{},
	)

	wf, err := workflow.New("pantry_recipe_wf", workflow.Chain(workflow.Start, node))
	if err != nil {
		return nil, fmt.Errorf("new pantry workflow: %w", err)
	}

	return agent.New(agent.Config{
		Name:        "PantryRecipeWorkflowAgent",
		Description: "Sáng tạo món ăn từ nguyên liệu có sẵn trong tủ lạnh của người dùng.",
		Run: func(ic agent.InvocationContext) iter.Seq2[*session.Event, error] {
			return wf.Run(ic)
		},
	})
}

// buildAdhocWorkflowAgent tạo workflow agent cho luồng gợi ý bữa ăn nhanh độc lập.
func (a *NutritionAgent) buildAdhocWorkflowAgent(_ context.Context) (agent.Agent, error) {
	systemPrompt, err := readPromptFile("prompts/adhoc.txt")
	if err != nil {
		return nil, fmt.Errorf("read adhoc prompt: %w", err)
	}

	node := workflow.NewDynamicNode(
		"adhoc_suggestion_workflow",
		func(nodeCtx agent.Context, userID string, _ func(*session.Event) error) (*GeneratedMealPlan, error) {
			if setErr := nodeCtx.State().Set("system_context", string(systemPrompt)); setErr != nil {
				return nil, fmt.Errorf("set adhoc system context: %w", setErr)
			}

			planJSON, genErr := workflow.RunNode[string](nodeCtx, a.generatorNode, userID)
			if genErr != nil {
				return nil, fmt.Errorf("adhoc generator: %w", genErr)
			}

			plan, parseErr := parseMealPlanFromNode(planJSON)
			if parseErr != nil {
				return nil, parseErr
			}

			a.putResult(nodeCtx.SessionID(), plan)
			return plan, nil
		},
		workflow.NodeConfig{},
	)

	wf, err := workflow.New("adhoc_suggestion_wf", workflow.Chain(workflow.Start, node))
	if err != nil {
		return nil, fmt.Errorf("new adhoc workflow: %w", err)
	}

	return agent.New(agent.Config{
		Name:        "AdhocSuggestionWorkflowAgent",
		Description: "Gợi ý nhanh 1-3 bữa ăn linh hoạt theo yêu cầu tức thời của người dùng.",
		Run: func(ic agent.InvocationContext) iter.Seq2[*session.Event, error] {
			return wf.Run(ic)
		},
	})
}
