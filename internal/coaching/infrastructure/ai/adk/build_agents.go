package adk

import (
	"context"
	"fmt"
	"os"

	"github.com/viethung213/gym-companion/internal/coaching/application/port"
	"google.golang.org/adk/v2/agent/llmagent"
	"google.golang.org/adk/v2/model"
	"google.golang.org/adk/v2/model/gemini"
	"google.golang.org/adk/v2/tool"
	"google.golang.org/adk/v2/workflow"
	"google.golang.org/genai"
)

type adkDeps struct {
	searchTool  tool.Tool
	prTool      tool.Tool
	clarifyTool tool.Tool
	replaceTool tool.Tool
	scaleTool   tool.Tool
	shiftTool   tool.Tool
	injurySkill tool.Toolset
}

func buildADKDeps(ctx context.Context, catalog port.ExerciseCatalogReader, sessionReader port.WorkoutSessionReader) (*adkDeps, error) {
	searchTool, err := makeSearchExercisesTool(catalog)
	if err != nil {
		return nil, fmt.Errorf("make search tool: %w", err)
	}

	prTool, err := makeGetExercisePRTool(sessionReader, "")
	if err != nil {
		return nil, fmt.Errorf("make pr tool: %w", err)
	}

	clarifyTool, err := makeAskClarifyingQuestionTool()
	if err != nil {
		return nil, fmt.Errorf("make clarify tool: %w", err)
	}

	replaceTool, err := makeReplaceInjuredExercisesTool(catalog)
	if err != nil {
		return nil, fmt.Errorf("make replace injured tool: %w", err)
	}

	scaleTool, err := makeScaleVolumeIntensityTool()
	if err != nil {
		return nil, fmt.Errorf("make scale volume tool: %w", err)
	}

	shiftTool, err := makeShiftSessionSlotsTool()
	if err != nil {
		return nil, fmt.Errorf("make shift slots tool: %w", err)
	}

	injurySkill, err := makeInjuryRecoverySkillToolset(ctx)
	if err != nil {
		return nil, fmt.Errorf("make injury skill: %w", err)
	}

	return &adkDeps{
		searchTool:  searchTool,
		prTool:      prTool,
		clarifyTool: clarifyTool,
		replaceTool: replaceTool,
		scaleTool:   scaleTool,
		shiftTool:   shiftTool,
		injurySkill: injurySkill,
	}, nil
}

func buildEvaluationResultSchema() *genai.Schema {
	return &genai.Schema{
		Type: genai.TypeObject,
		Properties: map[string]*genai.Schema{
			"is_valid": {Type: genai.TypeBoolean},
			"issues": {
				Type:  genai.TypeArray,
				Items: &genai.Schema{Type: genai.TypeString},
			},
		},
		Required: []string{"is_valid"},
	}
}

func (c *CoachingContextAgent) buildLLMNodes(_ context.Context, geminiModel model.LLM, deps *adkDeps) error {
	generatorInstruction, err := os.ReadFile("internal/coaching/infrastructure/ai/adk/prompts/generator.txt")
	if err != nil {
		return fmt.Errorf("read generator prompt: %w", err)
	}

	generatorAgent, err := llmagent.New(llmagent.Config{
		Name:                 "CoachGeneratorAgent",
		Description:          "Fitness expert generator agent using exercise and history tools.",
		Model:                geminiModel,
		Instruction:          string(generatorInstruction),
		Tools:                []tool.Tool{deps.searchTool, deps.prTool, deps.clarifyTool, deps.replaceTool, deps.scaleTool, deps.shiftTool},
		Toolsets:             []tool.Toolset{deps.injurySkill},
		OutputKey:            "generated_plan_text",
		BeforeModelCallbacks: beforeModelCallbacks("CoachGeneratorAgent", validateInputSafety),
		BeforeToolCallbacks:  []llmagent.BeforeToolCallback{validateToolExecution},
	})
	if err != nil {
		return fmt.Errorf("new generator agent: %w", err)
	}

	generatorNode, err := workflow.NewAgentNode(generatorAgent, workflow.NodeConfig{})
	if err != nil {
		return fmt.Errorf("new generator node: %w", err)
	}

	evaluatorInstruction, err := os.ReadFile("internal/coaching/infrastructure/ai/adk/prompts/evaluator.txt")
	if err != nil {
		return fmt.Errorf("read evaluator prompt: %w", err)
	}

	evaluatorAgent, err := llmagent.New(llmagent.Config{
		Name:                 "CoachEvaluatorAgent",
		Description:          "Final quality reviewer ensuring plan complies with phase limits.",
		Model:                geminiModel,
		Instruction:          string(evaluatorInstruction),
		OutputSchema:         buildEvaluationResultSchema(),
		BeforeModelCallbacks: beforeModelCallbacks("CoachEvaluatorAgent"),
	})
	if err != nil {
		return fmt.Errorf("new evaluator agent: %w", err)
	}

	evaluatorNode, err := workflow.NewAgentNode(evaluatorAgent, workflow.NodeConfig{})
	if err != nil {
		return fmt.Errorf("new evaluator node: %w", err)
	}

	c.generatorNode = generatorNode
	c.evaluatorNode = evaluatorNode
	return nil
}

// It creates the gemini model, all tools, LLM nodes, shared nodes, and workflow agents.
func (c *CoachingContextAgent) build(ctx context.Context) error {
	geminiModel, err := gemini.NewModel(ctx, "gemini-flash-latest", nil) // returns model.LLM
	if err != nil {
		return fmt.Errorf("new gemini model: %w", err)
	}

	deps, err := buildADKDeps(ctx, c.catalog, c.sessionReader)
	if err != nil {
		return err
	}

	if err := c.buildLLMNodes(ctx, geminiModel, deps); err != nil {
		return err
	}

	c.buildNodes()

	return c.buildWorkflowAgents(ctx)
}

func (c *CoachingContextAgent) buildWorkflowAgents(ctx context.Context) error {
	var err error

	c.initRoadmapWorkflowAgent, err = c.buildInitWorkflowAgent(ctx)
	if err != nil {
		return fmt.Errorf("build init workflow agent: %w", err)
	}

	c.suggestAdHocAgent, err = c.buildSuggestAdHocAgent(ctx)
	if err != nil {
		return fmt.Errorf("build suggest adhoc agent: %w", err)
	}

	c.regeneratePendingAgent, err = c.buildRegeneratePendingAgent(ctx)
	if err != nil {
		return fmt.Errorf("build regenerate agent: %w", err)
	}

	c.adaptiveCycleAgent, err = c.buildAdaptiveCycleAgent(ctx)
	if err != nil {
		return fmt.Errorf("build adaptive agent: %w", err)
	}

	return nil
}
