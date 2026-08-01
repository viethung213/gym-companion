package adk

import (
	"context"
	"errors"
	"fmt"
	"log"
	"sync"
	"time"

	"github.com/viethung213/gym-companion/internal/coaching/application/port"
	"github.com/viethung213/gym-companion/internal/coaching/domain/roadmap"
	"google.golang.org/adk/v2/agent"
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
	generatorNode *workflow.AgentNode
	evaluatorNode *workflow.AgentNode
	fetchNode     workflow.Node
	pendingNode   workflow.Node
	parseNode     workflow.Node

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

	// runner.Run yields only events, so this is the sole channel out of a node.
	mu      sync.Mutex
	results map[string]*PlanResult
	reasons map[string]string
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

	loadEnvFile()

	cca := &CoachingContextAgent{
		profileReader: profileReader,
		sessionReader: sessionReader,
		catalog:       catalogReader,
		roadmaps:      roadmaps,
		idgen:         idgen,
		validator:     newPlanValidator(catalogReader),
		results:       make(map[string]*PlanResult),
		reasons:       make(map[string]string),
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

// GenerateRoadmap satisfies port.CoachAgent.
func (c *CoachingContextAgent) GenerateRoadmap(ctx context.Context, userID string) (*roadmap.Roadmap, error) {
	res, err := c.runInitWorkflow(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("run initiate workflow: %w", err)
	}

	if res.Degraded {
		log.Printf("coaching: roadmap for user %s generated in DEGRADED mode (%d issues): %v",
			userID, len(res.Issues), res.Issues)
	}

	return c.mapToDomainRoadmap(ctx, res.Plan, res.Names, userID, time.Now().UTC())
}

// RegeneratePending satisfies port.CoachAgent. The workflow reads the pending
// sessions itself; results come back in the same order.
func (c *CoachingContextAgent) RegeneratePending(ctx context.Context, userID string, _ []string) ([]*roadmap.SessionPlanInfo, error) {
	res, err := c.runWorkflow(ctx, c.regeneratePendingAgent, userID, "")
	if err != nil {
		return nil, fmt.Errorf("run regenerate workflow: %w", err)
	}

	return c.mapToRegeneratedSessions(ctx, res.Plan, res.Names, userID, time.Now().UTC()), nil
}

// Adapt satisfies port.CoachAgent.
func (c *CoachingContextAgent) Adapt(ctx context.Context, userID, decisionReason string) ([]*roadmap.SessionPlanInfo, error) {
	res, err := c.runWorkflow(ctx, c.adaptiveCycleAgent, userID, decisionReason)
	if err != nil {
		return nil, fmt.Errorf("run adaptive workflow: %w", err)
	}

	return c.mapToRegeneratedSessions(ctx, res.Plan, res.Names, userID, time.Now().UTC()), nil
}

// SuggestAdHocSession satisfies port.CoachAgent.
func (c *CoachingContextAgent) SuggestAdHocSession(ctx context.Context, userID string, _ *port.AdHocHint) (port.SuggestedSession, error) {
	res, err := c.runWorkflow(ctx, c.suggestAdHocAgent, userID, "")
	if err != nil {
		return port.SuggestedSession{}, fmt.Errorf("run suggest adhoc workflow: %w", err)
	}

	if len(res.Plan.Weeks) == 0 || len(res.Plan.Weeks[0].Sessions) == 0 {
		return port.SuggestedSession{}, fmt.Errorf("%w: no session generated", ErrPlanGenerationFailed)
	}

	firstSess := res.Plan.Weeks[0].Sessions[0]
	return port.SuggestedSession{
		MuscleGroups: firstSess.TargetMuscleGroups,
		Prescription: c.mapPrescriptionToDomain(ctx, firstSess.Prescription, res.Names),
		Reasoning:    firstSess.Reasoning,
		EstimatedRPE: defaultEstimatedRPE,
	}, nil
}
