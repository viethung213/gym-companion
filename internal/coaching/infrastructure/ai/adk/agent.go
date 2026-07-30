package adk

import (
	"context"
	"encoding/json"
	"fmt"
	"iter"
	"os"
	"strings"
	"time"

	"github.com/google/uuid"
	"google.golang.org/adk/v2/agent"
	"google.golang.org/adk/v2/agent/llmagent"
	"google.golang.org/adk/v2/model/gemini"
	"google.golang.org/adk/v2/runner"
	"google.golang.org/adk/v2/session"
	"google.golang.org/adk/v2/tool"
	"google.golang.org/adk/v2/workflow"
	"google.golang.org/genai"

	"github.com/viethung213/gym-companion/internal/coaching/application/port"
	"github.com/viethung213/gym-companion/internal/coaching/domain/roadmap"
)

// Flow constants mirroring python prototype.
const (
	FlowInitiate4Week = "INITIATE_4_WEEK"
	FlowRegenerate    = "REGENERATE_PENDING"
	FlowAdaptiveCycle = "ADAPTIVE_CYCLE"
	FlowSignalHandler = "SIGNAL_HANDLER"
	FlowPostInjury    = "POST_INJURY_RECOVERY"
	FlowSuggestAdHoc  = "SUGGEST_AD_HOC_SESSION"
)

// compile-time verification
var _ port.CoachAgent = (*CoachingContextAgent)(nil)

// evaluationResultSchema defines the genai.Schema for Evaluator Agent.
var evaluationResultSchema = &genai.Schema{
	Type: genai.TypeObject,
	Properties: map[string]*genai.Schema{
		"is_valid": {
			Type: genai.TypeBoolean,
		},
		"issues": {
			Type: genai.TypeArray,
			Items: &genai.Schema{
				Type: genai.TypeString,
			},
		},
	},
	Required: []string{"is_valid"},
}

// CoachingContextAgent orchestrates the sequential ADK workflow.
type CoachingContextAgent struct {
	generatorNode *workflow.AgentNode
	evaluatorNode *workflow.AgentNode
	fetchNode     workflow.Node
	parseNode     workflow.Node

	initRoadmapWorkflowAgent agent.Agent
	defaultWorkflowAgent     agent.Agent
	suggestAdHocAgent        agent.Agent
	regeneratePendingAgent   agent.Agent
	adaptiveCycleAgent       agent.Agent

	profileReader port.UserProfileReader
	sessionReader port.WorkoutSessionReader
}

// NewCoachingContextAgent initializes LLM agents, tools, skills, callbacks, and workflow nodes.
func NewCoachingContextAgent(
	ctx context.Context,
	profileReader port.UserProfileReader,
	sessionReader port.WorkoutSessionReader,
	catalogReader port.ExerciseCatalogReader,
) (*CoachingContextAgent, error) {
	loadEnvFile()

	geminiModel, err := gemini.NewModel(ctx, "gemini-flash-latest", nil)
	if err != nil {
		return nil, fmt.Errorf("new gemini model: %w", err)
	}

	searchTool, err := makeSearchExercisesTool(catalogReader)
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

	replaceInjuredTool, err := makeReplaceInjuredExercisesTool(catalogReader)
	if err != nil {
		return nil, fmt.Errorf("make replace injured tool: %w", err)
	}

	scaleVolumeTool, err := makeScaleVolumeIntensityTool()
	if err != nil {
		return nil, fmt.Errorf("make scale volume tool: %w", err)
	}

	shiftSlotsTool, err := makeShiftSessionSlotsTool()
	if err != nil {
		return nil, fmt.Errorf("make shift slots tool: %w", err)
	}

	injurySkill, err := makeInjuryRecoverySkillToolset(ctx)
	if err != nil {
		return nil, fmt.Errorf("make injury skill: %w", err)
	}

	generatorInstruction, err := os.ReadFile("internal/coaching/infrastructure/ai/adk/prompts/generator.txt")
	if err != nil {
		return nil, fmt.Errorf("read generator prompt: %w", err)
	}

	generatorAgent, err := llmagent.New(llmagent.Config{
		Name:        "CoachGeneratorAgent",
		Description: "Fitness expert generator agent using exercise and history tools.",
		Model:       geminiModel,
		Instruction: string(generatorInstruction),
		Tools:                []tool.Tool{searchTool, prTool, clarifyTool, replaceInjuredTool, scaleVolumeTool, shiftSlotsTool},
		Toolsets:             []tool.Toolset{injurySkill},
		OutputKey:            "generated_plan_text",
		BeforeModelCallbacks: []llmagent.BeforeModelCallback{validateInputSafety},
		BeforeToolCallbacks:  []llmagent.BeforeToolCallback{validateToolExecution},
	})
	if err != nil {
		return nil, fmt.Errorf("new generator agent: %w", err)
	}

	generatorNode, err := workflow.NewAgentNode(generatorAgent, workflow.NodeConfig{})
	if err != nil {
		return nil, fmt.Errorf("new generator node: %w", err)
	}

	evaluatorInstruction, err := os.ReadFile("internal/coaching/infrastructure/ai/adk/prompts/evaluator.txt")
	if err != nil {
		return nil, fmt.Errorf("read evaluator prompt: %w", err)
	}

	evaluatorAgent, err := llmagent.New(llmagent.Config{
		Name:        "CoachEvaluatorAgent",
		Description: "Final quality reviewer ensuring plan complies with phase limits.",
		Model:       geminiModel,
		Instruction: string(evaluatorInstruction),
		OutputSchema: evaluationResultSchema,
	})
	if err != nil {
		return nil, fmt.Errorf("new evaluator agent: %w", err)
	}

	evaluatorNode, err := workflow.NewAgentNode(evaluatorAgent, workflow.NodeConfig{})
	if err != nil {
		return nil, fmt.Errorf("new evaluator node: %w", err)
	}

	cca := &CoachingContextAgent{
		generatorNode: generatorNode,
		evaluatorNode: evaluatorNode,
		profileReader: profileReader,
		sessionReader: sessionReader,
	}

	cca.fetchNode = workflow.NewFunctionNode("fetch_user_context", func(nodeCtx agent.Context, rawUserID string) (CoachInput, error) {
		userID := strings.TrimSpace(rawUserID)
		if userID == "" || len(userID) < 3 || userID == "chào" || strings.Contains(userID, " ") {
			userID = "user_default"
		}

		profile, getErr := profileReader.GetProfile(nodeCtx, userID)
		if getErr != nil {
			return CoachInput{}, fmt.Errorf("get profile: %w", getErr)
		}

		// Fetch only recent 7 days (max 3 items) to drastically reduce Prompt Tokens
		recent, getErr := sessionReader.GetRecentSessions(nodeCtx, userID, time.Now().AddDate(0, 0, -7))
		if getErr != nil {
			return CoachInput{}, fmt.Errorf("get recent sessions: %w", getErr)
		}
		if len(recent) > 3 {
			recent = recent[:3]
		}

		slots := make([]WorkoutSlot, len(profile.AvailableSlots))
		for i, s := range profile.AvailableSlots {
			slots[i] = WorkoutSlot{
				DayOfWeek: int(s.DayOfWeek),
				StartTime: s.StartTime,
				EndTime:   s.EndTime,
			}
		}

		injuries := make([]InjuryStatus, len(profile.ActiveInjuries))
		for i, inj := range profile.ActiveInjuries {
			injuries[i] = InjuryStatus{
				MuscleGroup: inj.MuscleGroup,
				Severity:    "MODERATE",
			}
		}

		adkSessions := make([]WorkoutSession, len(recent))
		for i, r := range recent {
			adkSessions[i] = WorkoutSession{
				SessionID:    r.SessionID,
				CompletedAt:  r.CompletedAt,
				MuscleGroups: nil,
				AverageRPE:   r.AverageRPE,
				TotalSets:    r.TotalSets,
				Aborted:      r.Aborted,
			}
		}

		return CoachInput{
			Flow:        FlowInitiate4Week,
			CurrentTime: time.Now().UTC().Format(time.RFC3339),
			Profile: UserProfile{
				UserID:                userID,
				WeightKg:              profile.WeightKg,
				PrimaryGoal:           profile.PrimaryGoal,
				AvailableEquipment:    profile.AvailableEquipment,
				PreferredMuscleGroups: profile.PreferredMuscleGroups,
				AvailableSlots:        slots,
				ActiveInjuries:        injuries,
			},
			RecentSessions: adkSessions,
		}, nil
	}, workflow.NodeConfig{})

	cca.parseNode = workflow.NewFunctionNode("parse_to_schema", func(nodeCtx agent.Context, planText string) (*GeneratedPlan, error) {
		var plan GeneratedPlan
		if err := json.Unmarshal([]byte(planText), &plan); err != nil {
			return nil, fmt.Errorf("unmarshal plan: %w", err)
		}
		if plan.UserID == "" || plan.UserID == "chào" {
			plan.UserID = "user_default"
		}
		return &plan, nil
	}, workflow.NodeConfig{})

	initDynamicNode := workflow.NewDynamicNode("init_roadmap_workflow", func(nodeCtx agent.Context, userID string, emit func(*session.Event) error) (*EvaluationResult, error) {
		coachInput, getErr := workflow.RunNode[CoachInput](nodeCtx, cca.fetchNode, userID)
		if getErr != nil {
			return nil, getErr
		}
		nodeCtx.State().Set("coach_input", coachInput)
		nodeCtx.State().Set("user_id", coachInput.Profile.UserID)

		planText, getErr := workflow.RunNode[string](nodeCtx, cca.generatorNode, coachInput)
		if getErr != nil {
			return nil, getErr
		}

		generated, getErr := workflow.RunNode[*GeneratedPlan](nodeCtx, cca.parseNode, planText)
		if getErr != nil {
			return nil, getErr
		}

		evaluatedMap, getErr := workflow.RunNode[map[string]any](nodeCtx, cca.evaluatorNode, generated)
		var evaluated *EvaluationResult
		if getErr == nil && evaluatedMap != nil {
			var isValid bool
			if v, ok := evaluatedMap["is_valid"].(bool); ok {
				isValid = v
			}
			var issues []string
			if rawIssues, ok := evaluatedMap["issues"].([]any); ok {
				for _, item := range rawIssues {
					if s, ok := item.(string); ok {
						issues = append(issues, s)
					}
				}
			}
			var finalPlan GeneratedPlan
			if generated != nil {
				finalPlan = *generated
				if finalPlan.UserID == "" || finalPlan.UserID == "chào" {
					finalPlan.UserID = coachInput.Profile.UserID
				}
			}
			evaluated = &EvaluationResult{
				IsValid: isValid,
				Issues:  issues,
				Plan:    finalPlan,
			}
			nodeCtx.State().Set("evaluation_result", evaluated)
		}
		return evaluated, getErr
	}, workflow.NodeConfig{})

	makeFlowDynamicNode := func(flowName string, flowType string) workflow.Node {
		return workflow.NewDynamicNode(flowName, func(nodeCtx agent.Context, userID string, emit func(*session.Event) error) (*GeneratedPlan, error) {
			coachInput, getErr := workflow.RunNode[CoachInput](nodeCtx, cca.fetchNode, userID)
			if getErr != nil {
				return nil, getErr
			}
			coachInput.Flow = flowType
			nodeCtx.State().Set("coach_input", coachInput)
			nodeCtx.State().Set("user_id", coachInput.Profile.UserID)

			planText, getErr := workflow.RunNode[string](nodeCtx, cca.generatorNode, coachInput)
			if getErr != nil {
				return nil, getErr
			}

			plan, getErr := workflow.RunNode[*GeneratedPlan](nodeCtx, cca.parseNode, planText)
			if getErr == nil && plan != nil {
				nodeCtx.State().Set("generated_plan", plan)
			}
			return plan, getErr
		}, workflow.NodeConfig{})
	}

	initWf, err := workflow.New("init_roadmap_wf", workflow.Chain(workflow.Start, initDynamicNode))
	if err != nil {
		return nil, fmt.Errorf("new init workflow: %w", err)
	}

	defaultWf, err := workflow.New("default_wf", workflow.Chain(workflow.Start, makeFlowDynamicNode("default_node", "DEFAULT_SESSION")))
	if err != nil {
		return nil, fmt.Errorf("new default workflow: %w", err)
	}

	suggestAdHocWf, err := workflow.New("suggest_adhoc_wf", workflow.Chain(workflow.Start, makeFlowDynamicNode("suggest_adhoc_node", FlowSuggestAdHoc)))
	if err != nil {
		return nil, fmt.Errorf("new suggest adhoc workflow: %w", err)
	}

	regenerateWf, err := workflow.New("regenerate_wf", workflow.Chain(workflow.Start, makeFlowDynamicNode("regenerate_node", FlowRegenerate)))
	if err != nil {
		return nil, fmt.Errorf("new regenerate workflow: %w", err)
	}

	adaptiveWf, err := workflow.New("adaptive_wf", workflow.Chain(workflow.Start, makeFlowDynamicNode("adaptive_node", FlowAdaptiveCycle)))
	if err != nil {
		return nil, fmt.Errorf("new adaptive workflow: %w", err)
	}

	initAgent, err := agent.New(agent.Config{
		Name:        "InitRoadmapWorkflowAgent",
		Description: "Orchestrates initial 4-week roadmap generation and quality evaluation.",
		Run: func(ic agent.InvocationContext) iter.Seq2[*session.Event, error] {
			return initWf.Run(ic)
		},
	})
	if err != nil {
		return nil, fmt.Errorf("new init agent: %w", err)
	}

	defaultAgent, err := agent.New(agent.Config{
		Name:        "DefaultWorkflowAgent",
		Description: "Orchestrates default coaching plan updates.",
		Run: func(ic agent.InvocationContext) iter.Seq2[*session.Event, error] {
			return defaultWf.Run(ic)
		},
	})
	if err != nil {
		return nil, fmt.Errorf("new default agent: %w", err)
	}

	suggestAdHocAgent, err := agent.New(agent.Config{
		Name:        "SuggestAdHocAgent",
		Description: "Single session suggestion with AG-UI/A2UI HITL question protocol.",
		Run: func(ic agent.InvocationContext) iter.Seq2[*session.Event, error] {
			return suggestAdHocWf.Run(ic)
		},
	})
	if err != nil {
		return nil, fmt.Errorf("new suggest adhoc agent: %w", err)
	}

	regenerateAgent, err := agent.New(agent.Config{
		Name:        "RegeneratePendingAgent",
		Description: "Rewrites specific pending workout sessions.",
		Run: func(ic agent.InvocationContext) iter.Seq2[*session.Event, error] {
			return regenerateWf.Run(ic)
		},
	})
	if err != nil {
		return nil, fmt.Errorf("new regenerate agent: %w", err)
	}

	adaptiveAgent, err := agent.New(agent.Config{
		Name:        "AdaptiveCycleAgent",
		Description: "Adapts workout sessions based on active injury/fatigue signals.",
		Run: func(ic agent.InvocationContext) iter.Seq2[*session.Event, error] {
			return adaptiveWf.Run(ic)
		},
	})
	if err != nil {
		return nil, fmt.Errorf("new adaptive agent: %w", err)
	}

	cca.initRoadmapWorkflowAgent = initAgent
	cca.defaultWorkflowAgent = defaultAgent
	cca.suggestAdHocAgent = suggestAdHocAgent
	cca.regeneratePendingAgent = regenerateAgent
	cca.adaptiveCycleAgent = adaptiveAgent

	return cca, nil
}

func (c *CoachingContextAgent) runInitWorkflow(ctx context.Context, userID string) (*EvaluationResult, error) {
	r, err := runner.NewInMemory("coaching-app", c.initRoadmapWorkflowAgent)
	if err != nil {
		return nil, fmt.Errorf("new runner: %w", err)
	}

	prompt := &genai.Content{
		Role: "user",
		Parts: []*genai.Part{
			{Text: userID},
		},
	}

	for _, runErr := range r.Run(ctx, userID, uuid.NewString(), prompt, agent.RunConfig{}) {
		if runErr != nil {
			return nil, fmt.Errorf("runner step error: %w", runErr)
		}
	}

	return &EvaluationResult{IsValid: true}, nil
}

func (c *CoachingContextAgent) runDefaultWorkflow(ctx context.Context, userID string) (*GeneratedPlan, error) {
	r, err := runner.NewInMemory("coaching-app", c.defaultWorkflowAgent)
	if err != nil {
		return nil, fmt.Errorf("new runner: %w", err)
	}

	prompt := &genai.Content{
		Role: "user",
		Parts: []*genai.Part{
			{Text: userID},
		},
	}

	for _, runErr := range r.Run(ctx, userID, uuid.NewString(), prompt, agent.RunConfig{}) {
		if runErr != nil {
			return nil, fmt.Errorf("runner step error: %w", runErr)
		}
	}

	return &GeneratedPlan{UserID: userID}, nil
}

// Agent returns the top-level ADK agent for initial 4-week roadmap generation.
func (c *CoachingContextAgent) Agent() agent.Agent {
	return c.initRoadmapWorkflowAgent
}

// DefaultAgent returns the ADK agent for fast ad-hoc/single session workflow.
func (c *CoachingContextAgent) DefaultAgent() agent.Agent {
	return c.defaultWorkflowAgent
}

// SuggestAdHocAgent returns the ADK agent for single session suggestion + HITL question.
func (c *CoachingContextAgent) SuggestAdHocAgent() agent.Agent {
	return c.suggestAdHocAgent
}

// RegeneratePendingAgent returns the ADK agent for regenerating pending sessions.
func (c *CoachingContextAgent) RegeneratePendingAgent() agent.Agent {
	return c.regeneratePendingAgent
}

// AdaptiveCycleAgent returns the ADK agent for adapting to injuries/signals.
func (c *CoachingContextAgent) AdaptiveCycleAgent() agent.Agent {
	return c.adaptiveCycleAgent
}

// GenerateRoadmap satisfies port.CoachAgent.
func (c *CoachingContextAgent) GenerateRoadmap(ctx context.Context, userID string) (*roadmap.Roadmap, error) {
	res, err := c.runInitWorkflow(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("run initiate workflow: %w", err)
	}

	return mapToDomainRoadmap(&res.Plan, time.Now())
}

// RegeneratePending satisfies port.CoachAgent.
func (c *CoachingContextAgent) RegeneratePending(ctx context.Context, userID string, sessionIDs []string) ([]*roadmap.SessionPlanInfo, error) {
	res, err := c.runDefaultWorkflow(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("run default workflow: %w", err)
	}

	return mapToDomainSessionInfos(res), nil
}

// Adapt satisfies port.CoachAgent.
func (c *CoachingContextAgent) Adapt(ctx context.Context, userID string, decisionReason string) ([]*roadmap.SessionPlanInfo, error) {
	res, err := c.runDefaultWorkflow(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("run default workflow: %w", err)
	}

	return mapToDomainSessionInfos(res), nil
}

// SuggestAdHocSession satisfies port.CoachAgent.
func (c *CoachingContextAgent) SuggestAdHocSession(ctx context.Context, userID string, hint port.AdHocHint) (port.SuggestedSession, error) {
	res, err := c.runDefaultWorkflow(ctx, userID)
	if err != nil {
		return port.SuggestedSession{}, fmt.Errorf("run default workflow: %w", err)
	}

	if len(res.Weeks) == 0 || len(res.Weeks[0].Sessions) == 0 {
		return port.SuggestedSession{}, fmt.Errorf("no session generated")
	}

	firstSess := res.Weeks[0].Sessions[0]
	return port.SuggestedSession{
		MuscleGroups: firstSess.TargetMuscleGroups,
		Prescription: mapPrescriptionToDomain(firstSess.Prescription),
		Reasoning:    firstSess.Reasoning,
		EstimatedRPE: 7.0,
	}, nil
}

// ── Domain mapping helpers ────────────────────────────────────────────────────

func mapToDomainRoadmap(plan *GeneratedPlan, now time.Time) (*roadmap.Roadmap, error) {
	roadmapID := uuid.NewString()
	roadmapInfo := &roadmap.Info{
		RoadmapID: roadmapID,
		UserID:    plan.UserID,
		Status:    roadmap.StatusActive,
		StartDate: now,
		EndDate:   now.AddDate(0, 0, 28),
		CreatedAt: now,
		UpdatedAt: now,
	}

	var weeks []*roadmap.WeekPlan
	for _, wp := range plan.Weeks {
		weekPlanID := uuid.NewString()
		weekInfo := &roadmap.WeekPlanInfo{
			WeekPlanID:      weekPlanID,
			RoadmapID:       roadmapID,
			UserID:          plan.UserID,
			WeekNumber:      int32(wp.WeekNumber),
			Phase:           roadmap.Phase(wp.Phase),
			TargetRPE:       float32((wp.TargetRPEMin + wp.TargetRPEMax) / 2.0),
			StartDate:       now.AddDate(0, 0, (wp.WeekNumber-1)*7),
			EndDate:         now.AddDate(0, 0, wp.WeekNumber*7),
			MuscleSplitType: "custom",
		}

		w, err := roadmap.NewWeekPlan(weekInfo)
		if err != nil {
			return nil, fmt.Errorf("new week plan: %w", err)
		}

		for _, sp := range wp.Sessions {
			scheduledTime, _ := time.Parse("2006-01-02", sp.ScheduledDate)
			sessionInfo := &roadmap.SessionPlanInfo{
				SessionPlanID:      uuid.NewString(),
				WeekPlanID:         weekPlanID,
				RoadmapID:          roadmapID,
				UserID:             plan.UserID,
				ScheduledDate:      scheduledTime,
				Status:             roadmap.SessionPlanStatusPending,
				TargetMuscleGroups: sp.TargetMuscleGroups,
				Prescription:       mapPrescriptionToDomain(sp.Prescription),
				Reasoning:          sp.Reasoning,
				GeneratedAt:        now,
			}
			s, err := roadmap.NewSessionPlan(sessionInfo, now)
			if err != nil {
				return nil, fmt.Errorf("new session plan: %w", err)
			}
			// DayPlan mapping
			dayInfo := &roadmap.DayPlanInfo{
				DayPlanID:     uuid.NewString(),
				WeekPlanID:    weekPlanID,
				RoadmapID:     roadmapID,
				UserID:        plan.UserID,
				ScheduledDate: scheduledTime,
			}
			d, err := roadmap.NewDayPlan(dayInfo)
			if err != nil {
				return nil, fmt.Errorf("new day plan: %w", err)
			}
			if err := d.AddSession(s); err != nil {
				return nil, fmt.Errorf("add session: %w", err)
			}
			if err := w.AddDay(d); err != nil {
				return nil, fmt.Errorf("add day: %w", err)
			}
		}
		weeks = append(weeks, w)
	}

	return roadmap.NewRoadmap(roadmapInfo, weeks, now)
}

func mapToDomainSessionInfos(plan *GeneratedPlan) []*roadmap.SessionPlanInfo {
	var infos []*roadmap.SessionPlanInfo
	for _, wp := range plan.Weeks {
		for _, sp := range wp.Sessions {
			scheduledTime, _ := time.Parse("2006-01-02", sp.ScheduledDate)
			infos = append(infos, &roadmap.SessionPlanInfo{
				SessionPlanID:      sp.SessionPlanID,
				ScheduledDate:      scheduledTime,
				TargetMuscleGroups: sp.TargetMuscleGroups,
				Prescription:       mapPrescriptionToDomain(sp.Prescription),
				Reasoning:          sp.Reasoning,
				Status:             roadmap.SessionPlanStatusPending,
			})
		}
	}
	return infos
}

func mapPrescriptionToDomain(p WorkoutPrescription) roadmap.WorkoutPrescription {
	return roadmap.WorkoutPrescription{
		WarmUps:       mapExercisesToDomain(p.WarmUps),
		MainExercises: mapExercisesToDomain(p.MainExercises),
		CoolDowns:     mapExercisesToDomain(p.CoolDowns),
	}
}

func mapExercisesToDomain(exs []PrescribedExercise) []roadmap.PrescribedExercise {
	out := make([]roadmap.PrescribedExercise, len(exs))
	for i, e := range exs {
		out[i] = roadmap.PrescribedExercise{
			ExerciseID:      e.ExerciseID,
			ExerciseName:    e.ExerciseName,
			TargetSets:      int32(e.TargetSets),
			TargetReps:      int32(e.TargetReps),
			TargetWeight:    float32(e.TargetWeightKg),
			TargetRPE:       float32(e.TargetRPE),
			RestSetSec:      int32(e.RestSetSec),
			RestExerciseSec: 90,
		}
	}
	return out
}

// loadEnvFile reads GOOGLE_API_KEY from root .env file if present.
func loadEnvFile() {
	data, err := os.ReadFile(".env")
	if err != nil {
		data, err = os.ReadFile("internal/coaching/infrastructure/ai/adk/.env")
		if err != nil {
			return
		}
	}
	lines := strings.Split(string(data), "\n")
	for _, line := range lines {
		parts := strings.SplitN(strings.TrimSpace(line), "=", 2)
		if len(parts) == 2 && os.Getenv(parts[0]) == "" {
			os.Setenv(parts[0], parts[1])
		}
	}
}
