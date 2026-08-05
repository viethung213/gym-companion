package adk

import (
	"context"
	"errors"
	"fmt"
	"strings"
)

const (
	maxGenerationAttempts = 3
	maxFeedbackIssues     = 20
)

var ErrNutritionPlanGenerationFailed = errors.New("nutrition plan generation failed")

type planAttemptFunc func(attempt int, priorIssues []string) (*GeneratedMealPlan, error)

func runWithRetries(
	ctx context.Context,
	v *planValidator,
	restrictions []string,
	attempt planAttemptFunc,
) (*PlanResult, error) {
	var priorIssues []string

	for n := 1; n <= maxGenerationAttempts; n++ {
		if err := ctx.Err(); err != nil {
			return nil, fmt.Errorf("nutrition plan generation cancelled before attempt %d: %w", n, err)
		}

		finalAttempt := n == maxGenerationAttempts

		plan, err := attempt(n, priorIssues)
		if err != nil {
			if finalAttempt {
				return nil, fmt.Errorf("%w: attempt %d: %w", ErrNutritionPlanGenerationFailed, n, err)
			}
			priorIssues = []string{"the previous attempt did not return a usable plan: " + err.Error()}
			continue
		}

		outcome, err := v.validate(ctx, plan, restrictions, finalAttempt)
		if err != nil {
			return nil, fmt.Errorf("validate nutrition plan: %w", err)
		}

		if !finalAttempt {
			if len(outcome.Issues) == 0 {
				return &PlanResult{Plan: outcome.Plan, Degraded: false}, nil
			}
			priorIssues = formatIssues(outcome.Issues)
			continue
		}

		// Final attempt: salvage valid options if available
		if len(outcome.Plan.Options) > 0 {
			return &PlanResult{Plan: outcome.Plan, Degraded: true}, nil
		}

		return nil, fmt.Errorf("%w: attempt 3 failed validation with issues: %s", ErrNutritionPlanGenerationFailed, strings.Join(outcome.Issues, "; "))
	}

	return nil, ErrNutritionPlanGenerationFailed
}

func formatIssues(issues []string) []string {
	if len(issues) > maxFeedbackIssues {
		issues = issues[:maxFeedbackIssues]
	}
	formatted := make([]string, 0, len(issues))
	for _, issue := range issues {
		formatted = append(formatted, "- "+issue)
	}
	return formatted
}
