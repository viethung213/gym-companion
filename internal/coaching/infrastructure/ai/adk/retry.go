package adk

import (
	"context"
	"errors"
	"fmt"
)

const (
	maxGenerationAttempts = 3
	// Past this, feedback stops being actionable and just inflates the prompt.
	maxFeedbackIssues = 20
)

// ErrPlanGenerationFailed indicates the agent could not produce a usable plan.
var ErrPlanGenerationFailed = errors.New("plan generation failed")

// planAttemptFunc produces one candidate plan; injected so retries need no LLM.
type planAttemptFunc func(attempt int, priorIssues []string) (*GeneratedPlan, error)

// runWithRetries feeds each round's defects into the next; the last one salvages.
//
// Blind re-rolls fail identically — inventing an ID is systematic, not random.
// Cancellation and catalog outages deliberately do not retry.
func runWithRetries(
	ctx context.Context,
	v *planValidator,
	attempt planAttemptFunc,
) (*PlanResult, error) {
	var priorIssues []string

	for n := 1; n <= maxGenerationAttempts; n++ {
		if err := ctx.Err(); err != nil {
			return nil, fmt.Errorf("plan generation cancelled before attempt %d: %w", n, err)
		}

		final := n == maxGenerationAttempts

		plan, err := attempt(n, priorIssues)
		if err != nil {
			if final {
				return nil, fmt.Errorf("%w: attempt %d: %w", ErrPlanGenerationFailed, n, err)
			}
			priorIssues = []string{"the previous attempt did not return a usable plan: " + err.Error()}
			continue
		}

		outcome, err := v.validate(ctx, plan, final)
		if err != nil {
			return nil, fmt.Errorf("validate exercises: %w", err) // infrastructure, not plan defect
		}

		if !final {
			if len(outcome.Issues) == 0 {
				return &PlanResult{Plan: outcome.Plan, Names: outcome.Names}, nil
			}
			priorIssues = formatIssues(outcome.Issues)
			continue
		}

		if planSessionCount(outcome.Plan) == 0 {
			return nil, fmt.Errorf("%w: no valid sessions survived %d attempts (%d issues, first: %s)",
				ErrPlanGenerationFailed, maxGenerationAttempts, len(outcome.Issues),
				firstIssue(outcome.Issues))
		}

		return &PlanResult{
			Plan:     outcome.Plan,
			Names:    outcome.Names,
			Degraded: outcome.Degraded,
			Issues:   formatIssues(outcome.Issues),
		}, nil
	}

	return nil, ErrPlanGenerationFailed // unreachable: the final iteration always returns
}

// formatIssues renders issues as feedback lines for the model.
func formatIssues(issues []planIssue) []string {
	if len(issues) == 0 {
		return nil
	}

	capped := issues
	truncated := 0
	if len(capped) > maxFeedbackIssues {
		truncated = len(capped) - maxFeedbackIssues
		capped = capped[:maxFeedbackIssues]
	}

	out := make([]string, 0, len(capped)+1)
	for _, iss := range capped {
		out = append(out, iss.String())
	}
	if truncated > 0 {
		out = append(out, fmt.Sprintf("(and %d further issues, omitted)", truncated))
	}
	return out
}

// firstIssue returns a printable form of the first issue, for error messages.
func firstIssue(issues []planIssue) string {
	if len(issues) == 0 {
		return "none reported"
	}
	return issues[0].String()
}
