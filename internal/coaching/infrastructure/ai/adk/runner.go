package adk

import (
	"context"
	"fmt"

	"github.com/google/uuid"
	"google.golang.org/adk/v2/agent"
	"google.golang.org/adk/v2/runner"
	"google.golang.org/adk/v2/workflow"
	"google.golang.org/genai"
)

// generateValidatedPlan adapts runWithRetries, keeping ADK inside the closure.
func (c *CoachingContextAgent) generateValidatedPlan(
	nodeCtx agent.Context,
	coachInput *CoachInput,
) (*PlanResult, error) {
	attempt := func(n int, priorIssues []string) (*GeneratedPlan, error) {
		input := *coachInput
		input.AttemptNumber = n
		input.PriorAttemptIssues = priorIssues

		// Republish each attempt: the safety callbacks read coach_input from state.
		if err := nodeCtx.State().Set("coach_input", input); err != nil {
			return nil, fmt.Errorf("record coach input: %w", err)
		}
		if err := nodeCtx.State().Set("user_id", input.Profile.UserID); err != nil {
			return nil, fmt.Errorf("record user id: %w", err)
		}

		planText, err := workflow.RunNode[string](nodeCtx, c.generatorNode, input)
		if err != nil {
			return nil, fmt.Errorf("generator: %w", err)
		}

		plan, err := workflow.RunNode[*GeneratedPlan](nodeCtx, c.parseNode, planText)
		if err != nil {
			return nil, fmt.Errorf("parse plan: %w", err)
		}
		return plan, nil
	}

	v := c.validator
	if n := len(coachInput.SessionsToRevise); n > 0 {
		v = v.expecting(n)
	}

	return runWithRetries(nodeCtx, v, attempt)
}

func (c *CoachingContextAgent) putResult(sessionID string, res *PlanResult) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.results[sessionID] = res
}

// takeResult always deletes, so an uncollected result cannot leak.
func (c *CoachingContextAgent) takeResult(sessionID string) *PlanResult {
	c.mu.Lock()
	defer c.mu.Unlock()
	res := c.results[sessionID]
	delete(c.results, sessionID)
	return res
}

func (c *CoachingContextAgent) putReason(sessionID, reason string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.reasons[sessionID] = reason
}

func (c *CoachingContextAgent) takeReason(sessionID string) string {
	c.mu.Lock()
	defer c.mu.Unlock()
	reason := c.reasons[sessionID]
	delete(c.reasons, sessionID)
	return reason
}

// runWorkflow executes a workflow agent and collects the plan its node produced.
func (c *CoachingContextAgent) runWorkflow(
	ctx context.Context,
	wfAgent agent.Agent,
	userID, reason string,
) (*PlanResult, error) {
	r, err := runner.NewInMemory("coaching-app", wfAgent)
	if err != nil {
		return nil, fmt.Errorf("new runner: %w", err)
	}

	sessionID := uuid.NewString()
	defer c.takeResult(sessionID) // ensure the slot is freed even on failure
	defer c.takeReason(sessionID)

	if reason != "" {
		c.putReason(sessionID, reason)
	}

	prompt := &genai.Content{
		Role:  "user",
		Parts: []*genai.Part{{Text: userID}},
	}

	for _, runErr := range r.Run(ctx, userID, sessionID, prompt, agent.RunConfig{}) {
		if runErr != nil {
			return nil, fmt.Errorf("runner step error: %w", runErr)
		}
	}

	res := c.takeResult(sessionID)
	if res == nil || res.Plan == nil || len(res.Plan.Weeks) == 0 {
		return nil, fmt.Errorf("%w: workflow produced no plan", ErrPlanGenerationFailed)
	}
	return res, nil
}

func (c *CoachingContextAgent) runInitWorkflow(ctx context.Context, userID string) (*PlanResult, error) {
	return c.runWorkflow(ctx, c.initRoadmapWorkflowAgent, userID, "")
}
