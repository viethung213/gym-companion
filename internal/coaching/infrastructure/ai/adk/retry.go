package adk

import (
	"context"
	"errors"
	"fmt"
	"log"
)

const (
	// maxGenerationAttempts bounds the whole loop, not one stage: each round
	// costs one generator call plus at most one reviewer call.
	maxGenerationAttempts = 3

	// Past this, feedback stops being actionable and just inflates the prompt.
	maxFeedbackIssues = 20

	// approvalScore is the score a reviewed plan must reach to ship.
	approvalScore = 70

	// minActionableConfidence is the confidence below which a rejection is
	// recorded but not acted on. An unsure reviewer sending the generator back
	// spends a round to replace one guess with another.
	minActionableConfidence = 0.5

	maxReviewScore = 100
)

// ErrPlanGenerationFailed indicates the agent could not produce a usable plan.
var ErrPlanGenerationFailed = errors.New("plan generation failed")

// errReviewUnusable means the reviewer's own output failed validation.
var errReviewUnusable = errors.New("review output unusable")

// attemptFeedback is everything the generator learns about its last try. The
// task and the user context are not here: they already live in CoachInput and
// duplicating them would let the two copies drift.
type attemptFeedback struct {
	PreviousPlan *GeneratedPlan
	Issues       []string    // deterministic defects from domain validation
	Review       *PlanReview // reviewer verdict, nil when validation failed first
}

// planAttemptFunc produces one candidate plan; injected so retries need no LLM.
type planAttemptFunc func(attempt int, fb *attemptFeedback) (*GeneratedPlan, error)

// planReviewFunc scores a validated plan; injected so the loop tests without an LLM.
// A nil func skips review entirely.
type planReviewFunc func(round int, plan *GeneratedPlan, report ValidationReport) (*PlanReview, error)

// runWithRetries drives generate → validate → review until a plan ships.
//
// Two gates, deliberately ordered. Domain validation runs first and a failure
// short-circuits straight back to the generator: paying for a review of a plan
// with fabricated exercise IDs buys nothing, and the reviewer would waste its
// feedback on defects that are already stated precisely.
//
// The final round salvages rather than failing, and the guardrail at the
// application layer remains the hard gate either way.
//
// Cancellation and catalog outages deliberately do not retry.
func runWithRetries(
	ctx context.Context,
	v *planValidator,
	attempt planAttemptFunc,
	review planReviewFunc,
) (*PlanResult, error) {
	var fb attemptFeedback

	for n := 1; n <= maxGenerationAttempts; n++ {
		if err := ctx.Err(); err != nil {
			return nil, fmt.Errorf("plan generation cancelled before attempt %d: %w", n, err)
		}

		final := n == maxGenerationAttempts

		plan, err := attempt(n, &fb)
		if err != nil {
			// Nothing the model can fix: a denied key, an unknown model, or a
			// rate limit that survived its own backoff budget. Regenerating
			// reproduces it in milliseconds, and the next prompt would carry an
			// API error dump dressed up as a plan defect.
			if isUnfixableByModel(err) {
				return nil, fmt.Errorf("%w: attempt %d hit an infrastructure failure: %w",
					ErrPlanGenerationFailed, n, err)
			}

			if final {
				return nil, fmt.Errorf("%w: attempt %d: %w", ErrPlanGenerationFailed, n, err)
			}
			fb = attemptFeedback{
				PreviousPlan: fb.PreviousPlan,
				Issues:       []string{"the previous attempt did not return a usable plan: " + err.Error()},
			}
			continue
		}

		outcome, err := v.validate(ctx, plan, final)
		if err != nil {
			return nil, fmt.Errorf("validate exercises: %w", err) // infrastructure, not plan defect
		}

		report := ValidationReport{
			Passed:   len(outcome.Issues) == 0,
			Degraded: outcome.Degraded,
			Issues:   formatIssues(outcome.Issues),
		}

		if !report.Passed && !final {
			// Gate 1 failed: send the precise defects back, skip the reviewer.
			fb = attemptFeedback{PreviousPlan: plan, Issues: report.Issues}
			continue
		}

		result := &PlanResult{
			Plan:     outcome.Plan,
			Names:    outcome.Names,
			Degraded: outcome.Degraded,
			Issues:   report.Issues,
		}

		if final && planSessionCount(outcome.Plan) == 0 {
			return nil, fmt.Errorf("%w: no valid sessions survived %d attempts (%d issues, first: %s)",
				ErrPlanGenerationFailed, maxGenerationAttempts, len(outcome.Issues),
				firstIssue(outcome.Issues))
		}

		if review == nil {
			return result, nil
		}

		verdict, err := review(n, outcome.Plan, report)
		if err != nil {
			// A reviewer that is down or incoherent must not block a plan that
			// already passed every deterministic gate.
			log.Printf("coaching: review unavailable on round %d, shipping unreviewed: %v", n, err)
			result.Degraded = true
			result.Issues = append(result.Issues, "plan shipped without review: "+err.Error())
			return result, nil
		}

		result.Review = verdict

		if reviewPasses(verdict) {
			return result, nil
		}

		if final {
			// Out of rounds. The reviewer's objections travel with the plan so
			// the caller can log or surface them; the guardrail still decides.
			result.Degraded = true
			result.Issues = append(result.Issues, reviewIssueLines(verdict)...)
			return result, nil
		}

		if verdict.Confidence < minActionableConfidence {
			log.Printf("coaching: review rejected round %d at confidence %.2f, too unsure to act on",
				n, verdict.Confidence)
			result.Degraded = true
			result.Issues = append(result.Issues, reviewIssueLines(verdict)...)
			return result, nil
		}

		// Gate 2 failed: reflect, carrying the plan being revised.
		fb = attemptFeedback{PreviousPlan: outcome.Plan, Review: verdict}
	}

	return nil, ErrPlanGenerationFailed // unreachable: the final iteration always returns
}

// reviewPasses reports whether a verdict clears both gates. Approval and score
// are checked together: a reviewer that approves while scoring 40 contradicts
// itself, and the safe reading of a contradiction is "not approved".
func reviewPasses(v *PlanReview) bool {
	return v != nil && v.Approved && v.Score >= approvalScore
}

// validateReview rejects a verdict the loop cannot act on. Without this a
// malformed review either sends the generator back with nothing to fix or, on
// an out-of-range score, silently blocks every plan.
func validateReview(v *PlanReview) error {
	if v == nil {
		return fmt.Errorf("%w: no verdict returned", errReviewUnusable)
	}
	if v.Score < 0 || v.Score > maxReviewScore {
		return fmt.Errorf("%w: score %d outside 0..%d", errReviewUnusable, v.Score, maxReviewScore)
	}
	if v.Confidence < 0 || v.Confidence > 1 {
		return fmt.Errorf("%w: confidence %.2f outside 0..1", errReviewUnusable, v.Confidence)
	}

	if reviewPasses(v) {
		return nil
	}

	// A rejection is a request to regenerate, so it must say what to change.
	if len(v.Feedback) == 0 {
		return fmt.Errorf("%w: rejected with no feedback", errReviewUnusable)
	}
	for i, note := range v.Feedback {
		if note.Fix == "" {
			return fmt.Errorf("%w: feedback[%d] (%s) has no fix", errReviewUnusable, i, note.Area)
		}
	}

	return nil
}

// reviewIssueLines renders a verdict as human-readable issue lines.
func reviewIssueLines(v *PlanReview) []string {
	if v == nil {
		return nil
	}

	out := make([]string, 0, len(v.Feedback)+1)
	out = append(out, fmt.Sprintf("review: score %d/%d, confidence %.2f, approved=%t",
		v.Score, maxReviewScore, v.Confidence, v.Approved))
	for _, note := range v.Feedback {
		out = append(out, fmt.Sprintf("review[%s]: %s → %s", note.Area, note.Detail, note.Fix))
	}
	return out
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
