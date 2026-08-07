package adk

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/google/uuid"
	"google.golang.org/adk/v2/agent"
	"google.golang.org/adk/v2/runner"
	"google.golang.org/adk/v2/workflow"
	"google.golang.org/genai"
)

// generateValidatedPlan adapts runWithRetries, keeping ADK inside the closure.
// withReview is false for flows that ship a single session, where a second LLM
// call costs more than the judgement is worth.
func (c *CoachingContextAgent) generateValidatedPlan(
	nodeCtx agent.Context,
	coachInput *CoachInput,
	withReview bool,
) (*PlanResult, error) {
	attempt := func(n int, fb *attemptFeedback) (*GeneratedPlan, error) {
		input := *coachInput
		input.AttemptNumber = n
		if fb != nil {
			input.PriorAttemptIssues = fb.Issues
			input.PreviousPlan = fb.PreviousPlan
			input.ReviewFeedback = fb.Review
		}

		// Republish each attempt: the safety callbacks read coach_input from state.
		if err := nodeCtx.State().Set("coach_input", input); err != nil {
			return nil, fmt.Errorf("record coach input: %w", err)
		}
		if err := nodeCtx.State().Set("user_id", input.Profile.UserID); err != nil {
			return nil, fmt.Errorf("record user id: %w", err)
		}

		planText, err := retryTransient(nodeCtx, "generator", func() (string, error) {
			return workflow.RunNode[string](nodeCtx, c.generatorNode, input)
		})
		if err != nil {
			return nil, fmt.Errorf("generator: %w", err)
		}

		plan, err := workflow.RunNode[*GeneratedPlan](nodeCtx, c.parseNode, planText)
		if err != nil {
			return nil, fmt.Errorf("parse plan: %w", err)
		}
		return plan, nil
	}

	var review planReviewFunc
	if withReview {
		review = c.reviewPlan(nodeCtx, coachInput)
	}

	v := c.validator
	if n := len(coachInput.SessionsToRevise); n > 0 {
		v = v.expecting(n)
	}

	return runWithRetries(nodeCtx, v, attempt, review)
}

// reviewPlan returns a planReviewFunc backed by the reviewer agent. The
// reviewer's own output is validated before the loop is allowed to act on it.
func (c *CoachingContextAgent) reviewPlan(
	nodeCtx agent.Context,
	coachInput *CoachInput,
) planReviewFunc {
	return func(round int, plan *GeneratedPlan, report ValidationReport, prior []ReviewNote) (*PlanReview, error) {
		if c.reviewerNode == nil {
			return nil, fmt.Errorf("%w: no reviewer configured", errReviewUnusable)
		}

		request := ReviewRequest{
			OriginalTask:     coachInput.Flow,
			UserContext:      coachInput.Profile,
			RecentSessions:   coachInput.RecentSessions,
			GeneratorOutput:  plan,
			ValidationResult: report,
			ReviewRound:      round,
			PreviousFeedback: prior,
		}

		raw, err := retryTransient(nodeCtx, "reviewer", func() (map[string]any, error) {
			return workflow.RunNode[map[string]any](nodeCtx, c.reviewerNode, request)
		})
		if err != nil {
			return nil, fmt.Errorf("reviewer: %w", err)
		}

		verdict := decodePlanReview(raw)
		if err := validateReview(verdict, prior); err != nil {
			return nil, err
		}
		return verdict, nil
	}
}

// decodePlanReview reads the reviewer's schema-constrained map. Absent or
// wrongly-typed fields fall to zero values, which validateReview then rejects
// rather than letting a half-parsed verdict steer the loop.
func decodePlanReview(raw map[string]any) *PlanReview {
	if raw == nil {
		return nil
	}

	var out PlanReview
	if v, ok := raw["approved"].(bool); ok {
		out.Approved = v
	}
	if v, ok := toFloat(raw["score"]); ok {
		out.Score = int(v)
	}
	if v, ok := toFloat(raw["confidence"]); ok {
		out.Confidence = v
	}

	for _, item := range objectsAt(raw, "feedback") {
		note := ReviewNote{}
		if s, ok := item["area"].(string); ok {
			note.Area = s
		}
		if s, ok := item["detail"].(string); ok {
			note.Detail = s
		}
		if s, ok := item["fix"].(string); ok {
			note.Fix = s
		}
		out.Feedback = append(out.Feedback, note)
	}

	for _, item := range objectsAt(raw, "previous_feedback") {
		outcome := FeedbackOutcome{}
		if s, ok := item["area"].(string); ok {
			outcome.Area = s
		}
		if b, ok := item["applied"].(bool); ok {
			outcome.Applied = b
		}
		if s, ok := item["evidence"].(string); ok {
			outcome.Evidence = s
		}
		out.PreviousFeedback = append(out.PreviousFeedback, outcome)
	}

	if rawStrings, ok := raw["notes"].([]any); ok {
		for _, item := range rawStrings {
			if s, ok := item.(string); ok {
				out.Notes = append(out.Notes, s)
			}
		}
	}

	return &out
}

// objectsAt reads an array-of-objects field, skipping anything of another shape.
func objectsAt(raw map[string]any, key string) []map[string]any {
	items, ok := raw[key].([]any)
	if !ok {
		return nil
	}

	out := make([]map[string]any, 0, len(items))
	for _, item := range items {
		if m, ok := item.(map[string]any); ok {
			out = append(out, m)
		}
	}
	return out
}

// toFloat accepts both JSON number shapes an any-typed map can hold.
func toFloat(v any) (float64, bool) {
	switch n := v.(type) {
	case float64:
		return n, true
	case int:
		return float64(n), true
	case int64:
		return float64(n), true
	case json.Number:
		f, err := n.Float64()
		return f, err == nil
	default:
		return 0, false
	}
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
