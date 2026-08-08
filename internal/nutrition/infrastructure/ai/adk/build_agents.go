package adk

import (
	"context"
	"embed"
	"encoding/json"
	"errors"
	"fmt"
	"iter"
	"log"
	"path/filepath"
	"strings"

	"google.golang.org/adk/v2/agent"
	"google.golang.org/adk/v2/agent/llmagent"
	"google.golang.org/adk/v2/model"
	"google.golang.org/adk/v2/session"
	"google.golang.org/adk/v2/tool"
	"google.golang.org/adk/v2/tool/functiontool"
	"google.golang.org/adk/v2/workflow"
)

//go:embed prompts/*.txt
var promptFS embed.FS

// readPromptFile đọc file prompt từ embedded filesystem promptFS.
func readPromptFile(path string) ([]byte, error) {
	filename := filepath.Base(path)
	return promptFS.ReadFile("prompts/" + filename)
}

// build khởi tạo Gemini model (với cơ chế Fallback tự động), LLM generator node và tất cả workflow agents.
func (a *NutritionAgent) build(ctx context.Context) error {
	geminiModel, err := NewFallbackLLMFromEnv(ctx)
	if err != nil {
		return fmt.Errorf("new fallback gemini model: %w", err)
	}

	if err := a.buildLLMNodes(ctx, geminiModel); err != nil {
		return err
	}

	return a.buildWorkflowAgents(ctx)
}

// buildLLMNodes đọc prompt generator.txt và khởi tạo LLM generator node.
// Cũng khởi tạo estimatorNode và insightNode cho các single-turn call.
func (a *NutritionAgent) buildLLMNodes(_ context.Context, geminiModel model.LLM) error {
	generatorInstruction, err := readPromptFile("prompts/generator.txt")
	if err != nil {
		return fmt.Errorf("read generator prompt: %w", err)
	}

	nutritionTools, err := a.buildADKTools()
	if err != nil {
		return fmt.Errorf("build adk tools: %w", err)
	}

	generatorAgent, err := llmagent.New(llmagent.Config{
		Name:        "NutritionGeneratorAgent",
		Description: "Chuyên gia dinh dưỡng AI: chọn thực phẩm sáng tạo, cân bằng Macro theo TDEE.",
		Model:       geminiModel,
		Instruction: string(generatorInstruction),
		Tools:       nutritionTools,
		OutputKey:   "generated_meal_plan_json",
	})
	if err != nil {
		return fmt.Errorf("new generator agent: %w", err)
	}

	generatorNode, err := workflow.NewAgentNode(generatorAgent, workflow.NodeConfig{})
	if err != nil {
		return fmt.Errorf("new generator node: %w", err)
	}

	a.generatorNode = generatorNode

	// Single-turn: EstimateNutrient
	estimatorAgent, err := buildSingleTurnAgent(
		"prompts/estimate_nutrient.txt",
		geminiModel,
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
		geminiModel,
		"NutritionInsightAgent",
		"Phân tích lịch sử dinh dưỡng và đề xuất hướng cải thiện cá nhân hóa.",
	)
	if err != nil {
		return fmt.Errorf("build insight agent: %w", err)
	}
	a.insightAgent = insightAgent

	return nil
}

// fetchCatalogArgs là args schema cho tool fetch_food_catalog.
type fetchCatalogArgs struct {
	Category string `json:"category"`
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
			Name:        "fetch_food_catalog",
			Description: "Tải danh mục thực phẩm đang hoạt động theo nhóm (PROTEIN, CARB, VEGGIE). Để trống category để lấy tất cả.",
		},
		func(ctx agent.Context, args fetchCatalogArgs) (interface{}, error) {
			return a.tools.FetchActiveFoodCatalog(ctx, args.Category)
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

	nutiFoodTool, err := functiontool.New(
		functiontool.Config{
			Name:        "suggest_nutifood_supplement",
			Description: "Gợi ý các sản phẩm NutiFood bổ sung phù hợp với thực đơn hiện tại.",
		},
		func(ctx agent.Context, _ struct{}) (interface{}, error) {
			return a.tools.SuggestNutiFoodSupplement(ctx)
		},
	)
	if err != nil {
		return nil, fmt.Errorf("make nutifood tool: %w", err)
	}

	return []tool.Tool{fetchTool, macroTool, nutiFoodTool}, nil
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
			log.Printf("nutrition adk: post-workout recalibration plan generated for session %s", nodeCtx.SessionID())
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
