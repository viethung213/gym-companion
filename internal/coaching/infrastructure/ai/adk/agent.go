package adk

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/google/uuid"
	"google.golang.org/adk/v2/agent"
	"google.golang.org/adk/v2/agent/llmagent"
	"google.golang.org/adk/v2/model/gemini"
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

	initRoadmapWorkflowNode workflow.Node
	defaultWorkflowNode     workflow.Node

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

	geminiModel, err := gemini.NewModel(ctx, "gemini-2.5-flash", nil)
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

	skillToolset, err := makeCoachingSkillToolset(ctx)
	if err != nil {
		return nil, fmt.Errorf("make skill toolset: %w", err)
	}

	generatorAgent, err := llmagent.New(llmagent.Config{
		Name:        "CoachGeneratorAgent",
		Description: "Fitness expert generator agent using exercise and history tools.",
		Model:       geminiModel,
		Instruction: `You are CoachGeneratorAgent — expert AI fitness coach.
Your task is to generate or adapt a workout plan based on the input flow.
You must always load the 'coaching-roadmap-rules' skill to read the exact phase rules and schema structure before writing your output.
Steps:
1. Call search_exercises to find movements.
2. Call get_exercise_pr to fetch user PRs and set target weights safely.
3. Load the coaching-roadmap-rules skill to verify phase target RPE and output schema.
4. Write the final response in clean JSON matching the schema from the skill. No prose.`,
		Tools:                []tool.Tool{searchTool, prTool},
		Toolsets:             []tool.Toolset{skillToolset},
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

	evaluatorAgent, err := llmagent.New(llmagent.Config{
		Name:        "CoachEvaluatorAgent",
		Description: "Final quality reviewer ensuring plan complies with phase limits.",
		Model:       geminiModel,
		Instruction: `You are CoachEvaluatorAgent — final quality reviewer.
Compare the GeneratedPlan input against the business rules:
1. Exact phase sequence: ACCUMULATION -> OVERLOAD -> PEAK -> DELOAD.
2. RPE ranges match phase targets.
3. Deload week volume is <= 70% of Peak week.
4. No sessions target active injuries.
Output only JSON matching the EvaluationResult schema.`,
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

	cca.fetchNode = workflow.NewFunctionNode("fetch_user_context", func(nodeCtx agent.Context, userID string) (CoachInput, error) {
		profile, getErr := profileReader.GetProfile(nodeCtx, userID)
		if getErr != nil {
			return CoachInput{}, fmt.Errorf("get profile: %w", getErr)
		}

		recent, getErr := sessionReader.GetRecentSessions(nodeCtx, userID, time.Now().AddDate(0, 0, -30))
		if getErr != nil {
			return CoachInput{}, fmt.Errorf("get recent sessions: %w", getErr)
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
				UserID:                profile.UserID,
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
		return &plan, nil
	}, workflow.NodeConfig{})

	cca.initRoadmapWorkflowNode = workflow.NewDynamicNode("init_roadmap_workflow", func(nodeCtx agent.Context, userID string, emit func(*session.Event) error) (*EvaluationResult, error) {
		coachInput, getErr := workflow.RunNode[CoachInput](nodeCtx, cca.fetchNode, userID)
		if getErr != nil {
			return nil, getErr
		}
		nodeCtx.State().Set("coach_input", coachInput)
		nodeCtx.State().Set("user_id", userID)

		planText, getErr := workflow.RunNode[string](nodeCtx, cca.generatorNode, coachInput)
		if getErr != nil {
			return nil, getErr
		}

		generated, getErr := workflow.RunNode[*GeneratedPlan](nodeCtx, cca.parseNode, planText)
		if getErr != nil {
			return nil, getErr
		}

		evaluated, getErr := workflow.RunNode[*EvaluationResult](nodeCtx, cca.evaluatorNode, generated)
		return evaluated, getErr
	}, workflow.NodeConfig{})

	cca.defaultWorkflowNode = workflow.NewDynamicNode("default_workflow", func(nodeCtx agent.Context, userID string, emit func(*session.Event) error) (*GeneratedPlan, error) {
		coachInput, getErr := workflow.RunNode[CoachInput](nodeCtx, cca.fetchNode, userID)
		if getErr != nil {
			return nil, getErr
		}
		nodeCtx.State().Set("coach_input", coachInput)
		nodeCtx.State().Set("user_id", userID)

		planText, getErr := workflow.RunNode[string](nodeCtx, cca.generatorNode, coachInput)
		if getErr != nil {
			return nil, getErr
		}

		return workflow.RunNode[*GeneratedPlan](nodeCtx, cca.parseNode, planText)
	}, workflow.NodeConfig{})

	return cca, nil
}

// GenerateRoadmap satisfies port.CoachAgent.
func (c *CoachingContextAgent) GenerateRoadmap(ctx context.Context, userID string) (*roadmap.Roadmap, error) {
	ic := agent.NewContext(nil).WithAgentContext(ctx)
	res, err := workflow.RunNode[*EvaluationResult](ic, c.initRoadmapWorkflowNode, userID)
	if err != nil {
		return nil, fmt.Errorf("run initiate workflow: %w", err)
	}

	return mapToDomainRoadmap(&res.Plan, time.Now())
}

// RegeneratePending satisfies port.CoachAgent.
func (c *CoachingContextAgent) RegeneratePending(ctx context.Context, userID string, sessionIDs []string) ([]*roadmap.SessionPlanInfo, error) {
	ic := agent.NewContext(nil).WithAgentContext(ctx)
	res, err := workflow.RunNode[*GeneratedPlan](ic, c.defaultWorkflowNode, userID)
	if err != nil {
		return nil, fmt.Errorf("run default workflow: %w", err)
	}

	return mapToDomainSessionInfos(res), nil
}

// Adapt satisfies port.CoachAgent.
func (c *CoachingContextAgent) Adapt(ctx context.Context, userID string, decisionReason string) ([]*roadmap.SessionPlanInfo, error) {
	ic := agent.NewContext(nil).WithAgentContext(ctx)
	res, err := workflow.RunNode[*GeneratedPlan](ic, c.defaultWorkflowNode, userID)
	if err != nil {
		return nil, fmt.Errorf("run default workflow: %w", err)
	}

	return mapToDomainSessionInfos(res), nil
}

// SuggestAdHocSession satisfies port.CoachAgent.
func (c *CoachingContextAgent) SuggestAdHocSession(ctx context.Context, userID string, hint port.AdHocHint) (port.SuggestedSession, error) {
	// Suggest a single session
	ic := agent.NewContext(nil).WithAgentContext(ctx)
	res, err := workflow.RunNode[*GeneratedPlan](ic, c.defaultWorkflowNode, userID)
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
