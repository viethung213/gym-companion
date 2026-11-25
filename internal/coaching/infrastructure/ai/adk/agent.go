package adk

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"sync"
	"time"

	"github.com/viethung213/gym-companion/internal/coaching/application/port"
	"github.com/viethung213/gym-companion/internal/coaching/domain/roadmap"
	"github.com/viethung213/gym-companion/internal/coaching/domain/service"
	"google.golang.org/adk/v2/agent"
	"google.golang.org/adk/v2/model"
	"google.golang.org/adk/v2/workflow"
)

const (
	FlowInitiate4Week = "INITIATE_4_WEEK"
	FlowRegenerate    = "REGENERATE_PENDING"
	FlowAdaptiveCycle = "ADAPTIVE_CYCLE"
	FlowSignalHandler = "SIGNAL_HANDLER"
	FlowPostInjury    = "POST_INJURY_RECOVERY"
	FlowSuggestAdHoc  = "SUGGEST_AD_HOC_SESSION"
)

const (
	defaultRestExerciseSec = 90
	defaultMuscleSplitType = "custom"
	defaultEstimatedRPE    = 7.0
)

// compile-time verification
var _ port.CoachAgent = (*CoachingContextAgent)(nil)

type CoachingContextAgent struct {
	generatorNode      *workflow.AgentNode
	adhocGeneratorNode *workflow.AgentNode
	reviewerNode       *workflow.AgentNode
	fetchNode          workflow.Node
	pendingNode        workflow.Node
	parseNode          workflow.Node

	initRoadmapWorkflowAgent agent.Agent
	suggestAdHocAgent        agent.Agent
	regeneratePendingAgent   agent.Agent
	adaptiveCycleAgent       agent.Agent

	profileReader port.UserProfileReader
	sessionReader port.WorkoutSessionReader
	catalog       port.ExerciseCatalogReader
	roadmaps      port.RoadmapRepository
	idgen         port.IDGenerator
	validator     *planValidator
	llm           model.LLM

	// runner.Run yields only events, so this is the sole channel out of a node.
	mu      sync.Mutex
	results map[string]*PlanResult
	reasons map[string]string
	hints   map[string]*port.AdHocHint
	// planStarts holds the start date of an initial roadmap planned ahead.
	planStarts map[string]time.Time
}

// NewCoachingContextAgent initializes LLM agents, tools, skills, callbacks, and workflow nodes.
func NewCoachingContextAgent(
	ctx context.Context,
	profileReader port.UserProfileReader,
	sessionReader port.WorkoutSessionReader,
	catalogReader port.ExerciseCatalogReader,
	roadmaps port.RoadmapRepository,
	idgen port.IDGenerator,
) (*CoachingContextAgent, error) {
	if idgen == nil {
		return nil, errors.New("id generator is required")
	}

	cca := &CoachingContextAgent{
		profileReader: profileReader,
		sessionReader: sessionReader,
		catalog:       catalogReader,
		roadmaps:      roadmaps,
		idgen:         idgen,
		validator:     newPlanValidator(catalogReader),
		results:       make(map[string]*PlanResult),
		reasons:       make(map[string]string),
		hints:         make(map[string]*port.AdHocHint),
		planStarts:    make(map[string]time.Time),
	}

	if err := cca.build(ctx); err != nil {
		return nil, err
	}

	return cca, nil
}

// initial 4-week roadmap generation.
func (c *CoachingContextAgent) Agent() agent.Agent {
	return c.initRoadmapWorkflowAgent
}

// SuggestAdHocAgent
func (c *CoachingContextAgent) SuggestAdHocAgent() agent.Agent {
	return c.suggestAdHocAgent
}

// RegeneratePendingAgent
func (c *CoachingContextAgent) RegeneratePendingAgent() agent.Agent {
	return c.regeneratePendingAgent
}

// AdaptiveCycleAgent
func (c *CoachingContextAgent) AdaptiveCycleAgent() agent.Agent {
	return c.adaptiveCycleAgent
}

// GenerateRoadmap satisfies port.CoachAgent: a 4-week draft starting now.
func (c *CoachingContextAgent) GenerateRoadmap(ctx context.Context, userID string) (*roadmap.Roadmap, error) {
	return c.generateRoadmap(ctx, userID, time.Time{})
}

// generateRoadmap drafts a roadmap starting now, or on startDate when set.
func (c *CoachingContextAgent) generateRoadmap(ctx context.Context, userID string, startDate time.Time) (*roadmap.Roadmap, error) {
	// Provisional dating; the application stamps the authoritative planning
	// time with its clock (Roadmap.StampPlannedAt).
	plannedAt := time.Now().UTC()
	res, err := c.runWorkflow(ctx, c.initRoadmapWorkflowAgent, userID, runOptions{planStart: startDate})
	if err != nil {
		return nil, fmt.Errorf("run initiate workflow: %w", err)
	}

	if res.Degraded {
		slog.WarnContext(ctx, "Roadmap generated in DEGRADED mode",
			slog.String("user_id", userID),
			slog.Int("issue_count", len(res.Issues)),
			slog.Any("issues", res.Issues))
	}

	weekStart := res.WeekStart
	if weekStart.IsZero() && !startDate.IsZero() {
		// No slot schedule (the athlete picked no slot): a block planned ahead
		// still starts the day it was planned for.
		weekStart = roadmap.CalendarDate(startDate, time.UTC)
	}
	rm, err := c.mapToDomainRoadmap(ctx, res.Plan, res.Names, res.Primaries, userID, plannedAt, weekStart)
	if err != nil {
		return nil, err
	}
	rm.SetTimezone(res.Timezone)
	return rm, nil
}

// RegeneratePending satisfies port.CoachAgent. The workflow reads the pending
// sessions itself; results come back in the same order.
func (c *CoachingContextAgent) RegeneratePending(ctx context.Context, userID string, _ []string) ([]*roadmap.SessionPlanInfo, error) {
	res, err := c.runWorkflow(ctx, c.regeneratePendingAgent, userID, runOptions{})
	if err != nil {
		return nil, fmt.Errorf("run regenerate workflow: %w", err)
	}

	return c.mapToRegeneratedSessions(ctx, res.Plan, res.Names, res.Primaries, userID, time.Now().UTC())
}

// Adapt satisfies port.CoachAgent.
func (c *CoachingContextAgent) Adapt(
	ctx context.Context, userID string, decision *service.AdaptationDecision,
) ([]*roadmap.SessionPlanInfo, error) {
	res, err := c.runWorkflow(ctx, c.adaptiveCycleAgent, userID, runOptions{reason: adaptationInstruction(decision)})
	if err != nil {
		return nil, fmt.Errorf("run adaptive workflow: %w", err)
	}

	return c.mapToRegeneratedSessions(ctx, res.Plan, res.Names, res.Primaries, userID, time.Now().UTC())
}

// SuggestAdHocSession satisfies port.CoachAgent.
func (c *CoachingContextAgent) SuggestAdHocSession(ctx context.Context, userID string, hint *port.AdHocHint) (port.SuggestedSession, error) {
	res, err := c.runAdHocWorkflow(ctx, userID, hint)
	if err != nil {
		return port.SuggestedSession{}, fmt.Errorf("run suggest adhoc workflow: %w", err)
	}

	if len(res.Plan.Weeks) == 0 || len(res.Plan.Weeks[0].Sessions) == 0 {
		return port.SuggestedSession{}, fmt.Errorf("%w: no session generated", ErrPlanGenerationFailed)
	}

	firstSess := res.Plan.Weeks[0].Sessions[0]
	muscles, err := sessionTargetMuscles(firstSess.Prescription, res.Primaries)
	if err != nil {
		return port.SuggestedSession{}, err
	}
	return port.SuggestedSession{
		MuscleGroups: muscles,
		Prescription: c.mapPrescriptionToDomain(ctx, firstSess.Prescription, res.Names),
		Reasoning:    firstSess.Reasoning,
		EstimatedRPE: defaultEstimatedRPE,
	}, nil
}
