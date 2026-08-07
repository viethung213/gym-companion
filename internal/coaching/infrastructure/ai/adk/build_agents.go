package adk

import (
	"context"
	"fmt"
	"os"

	"github.com/viethung213/gym-companion/internal/coaching/application/port"
	"google.golang.org/adk/v2/agent/llmagent"
	"google.golang.org/adk/v2/model"
	"google.golang.org/adk/v2/tool"
	"google.golang.org/adk/v2/workflow"
	"google.golang.org/genai"
)

// coachModel is the Gemini model both the generator and the reviewer run on.
const coachModel = "gemini-3.5-flash-lite"

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

	prTool, err := makeGetExercisePRTool(sessionReader)
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
		AfterModelCallbacks:  afterModelCallbacks("CoachGeneratorAgent"),
		BeforeToolCallbacks:  []llmagent.BeforeToolCallback{validateToolExecution},
	})
	if err != nil {
		return fmt.Errorf("new generator agent: %w", err)
	}

	generatorNode, err := workflow.NewAgentNode(generatorAgent, workflow.NodeConfig{})
	if err != nil {
		return fmt.Errorf("new generator node: %w", err)
	}

	reviewerInstruction, err := os.ReadFile("internal/coaching/infrastructure/ai/adk/prompts/evaluator.txt")
	if err != nil {
		return fmt.Errorf("read reviewer prompt: %w", err)
	}

	// No tools: a reviewer that could call search_exercises would start
	// proposing replacements, which is the generator's job.
	reviewerAgent, err := llmagent.New(llmagent.Config{
		Name:                 "CoachReviewerAgent",
		Description:          "Scores a validated plan against the athlete's context and returns actionable feedback.",
		Model:                geminiModel,
		Instruction:          string(reviewerInstruction),
		OutputSchema:         buildPlanReviewSchema(),
		BeforeModelCallbacks: beforeModelCallbacks("CoachReviewerAgent"),
		AfterModelCallbacks:  afterModelCallbacks("CoachReviewerAgent"),
	})
	if err != nil {
		return fmt.Errorf("new reviewer agent: %w", err)
	}

	reviewerNode, err := workflow.NewAgentNode(reviewerAgent, workflow.NodeConfig{})
	if err != nil {
		return fmt.Errorf("new reviewer node: %w", err)
	}

	c.generatorNode = generatorNode
	c.reviewerNode = reviewerNode
	return nil
}

// buildPlanReviewSchema mirrors PlanReview. There is deliberately no plan
// field: the schema is what makes "review only, never rewrite" structural
// rather than a request the model may ignore.
func buildPlanReviewSchema() *genai.Schema {
	return &genai.Schema{
		Type: genai.TypeObject,
		Properties: map[string]*genai.Schema{
			"approved":   {Type: genai.TypeBoolean},
			"score":      {Type: genai.TypeInteger},
			"confidence": {Type: genai.TypeNumber},
			// Must-fix only. Notes exists so praise has somewhere to go that is
			// not a "fix", which would pass validation while telling the
			// generator nothing to do.
			"feedback": {
				Type: genai.TypeArray,
				Items: &genai.Schema{
					Type: genai.TypeObject,
					Properties: map[string]*genai.Schema{
						"area":   {Type: genai.TypeString},
						"detail": {Type: genai.TypeString},
						"fix":    {Type: genai.TypeString},
					},
					Required: []string{"area", "detail", "fix"},
				},
			},
			"notes": {
				Type:  genai.TypeArray,
				Items: &genai.Schema{Type: genai.TypeString},
			},
			// One entry per item the previous round asked for. Evidence is
			// required either way: "applied" without a location is a claim.
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

// It creates the gemini model, all tools, LLM nodes, shared nodes, and workflow agents.
func (c *CoachingContextAgent) build(ctx context.Context) error {
	// Pinned rather than an alias: "gemini-flash-latest" silently moved to
	// gemini-3.6-flash, and a model change that nobody chose is a model change
	// nobody measured.
	geminiModel, err := NewFallbackLLMFromEnv(ctx)
	if err != nil {
		return fmt.Errorf("new fallback gemini model: %w", err)
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
