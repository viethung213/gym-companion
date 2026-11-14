package adk

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"strings"
)

const (
	maxGenerationAttempts   = 2
	maxFeedbackIssues       = 20
	approvalScore           = 70
	minActionableConfidence = 0.5
	maxReviewScore          = 100
)

var (
	ErrNutritionPlanGenerationFailed = errors.New("nutrition plan generation failed")
	errReviewUnusable                = errors.New("review output unusable")
)

// attemptFeedback lưu trữ kết quả và lỗi/phản biện của lượt trước gửi cho Generator.
type attemptFeedback struct {
	PreviousPlan *GeneratedMealPlan
	Issues       []string        // Lỗi tất định từ validator
	Review       *MealPlanReview // Phản biện từ Reviewer LLM
}

type planAttemptFunc func(attempt int, fb *attemptFeedback) (*GeneratedMealPlan, error)

type planReviewFunc func(round int, plan *GeneratedMealPlan, report ValidationReport, prior []ReviewNote) (*MealPlanReview, error)

// runWithRetries điều phối chu trình: Generator -> Validate (Gate 1) -> Reviewer (Gate 2) -> Reflection.
func runWithRetries(
	ctx context.Context,
	v *planValidator,
	availableIngredients []FoodNutrientDTO,
	restrictions []string,
	attempt planAttemptFunc,
	review planReviewFunc,
) (*PlanResult, error) {
	var fb attemptFeedback
	var priorFeedback []ReviewNote
	var reviewLogs []ReviewRoundLog

	for n := 1; n <= maxGenerationAttempts; n++ {
		if err := ctx.Err(); err != nil {
			return nil, fmt.Errorf("nutrition plan generation cancelled before attempt %d: %w", n, err)
		}

		finalAttempt := n == maxGenerationAttempts

		plan, err := attempt(n, &fb)
		if err != nil {
			if finalAttempt {
				return nil, fmt.Errorf("%w: attempt %d: %w", ErrNutritionPlanGenerationFailed, n, err)
			}
			fb = attemptFeedback{
				PreviousPlan: fb.PreviousPlan,
				Issues:       []string{"the previous attempt did not return a usable plan: " + err.Error()},
			}
			continue
		}

		outcome, err := v.validate(ctx, plan, availableIngredients, restrictions, finalAttempt)
		if err != nil {
			return nil, fmt.Errorf("validate nutrition plan: %w", err)
		}

		report := ValidationReport{
			Passed:   len(outcome.Issues) == 0,
			Degraded: !finalAttempt && len(outcome.Issues) > 0,
			Issues:   formatIssues(outcome.Issues),
		}

		// Gate 1: Lỗi tất định (sai catalog, dính dị ứng) -> yêu cầu sửa ngay, không gọi Reviewer tốn token
		if !report.Passed && !finalAttempt {
			fb = attemptFeedback{PreviousPlan: plan, Issues: report.Issues}
			continue
		}

		result := &PlanResult{
			Plan:       outcome.Plan,
			Degraded:   false,
			Issues:     report.Issues,
			Attempts:   n,
			ReviewLogs: reviewLogs,
		}

		if finalAttempt && (outcome.Plan == nil || len(outcome.Plan.Options) == 0) {
			return nil, fmt.Errorf("%w: attempt 2 failed validation with issues: %s",
				ErrNutritionPlanGenerationFailed, strings.Join(outcome.Issues, "; "))
		}

		// Nếu không cấu hình reviewer (ví dụ luồng adhoc nhanh)
		if review == nil {
			if finalAttempt && len(outcome.Issues) > 0 {
				result.Degraded = true
			}
			return result, nil
		}

		// Gate 2: Reviewer LLM phản biện chất lượng ẩm thực & độ khả thi
		verdict, err := review(n, clonePlan(outcome.Plan), report, priorFeedback)
		if err != nil {
			slog.Warn("Nutrition review unavailable, shipping unreviewed", slog.Int("round", n), slog.Any("error", err))
			result.Degraded = true
			result.Issues = append(result.Issues, "plan shipped without review: "+err.Error())
			return result, nil
		}

		roundLog := ReviewRoundLog{
			Round:      n,
			Approved:   verdict.Approved,
			Score:      verdict.Score,
			Confidence: verdict.Confidence,
			Feedback:   verdict.Feedback,
			Notes:      verdict.Notes,
		}
		reviewLogs = append(reviewLogs, roundLog)
		result.Review = verdict
		result.ReviewLogs = reviewLogs

		if reviewPasses(verdict) {
			return result, nil
		}

		if finalAttempt {
			result.Degraded = true
			result.Issues = append(result.Issues, reviewIssueLines(verdict)...)
			return result, nil
		}

		if verdict.Confidence < minActionableConfidence {
			slog.Warn("Nutrition review rejected, confidence too low to act on",
				slog.Int("round", n), slog.Float64("confidence", verdict.Confidence))
			result.Degraded = true
			result.Issues = append(result.Issues, reviewIssueLines(verdict)...)
			return result, nil
		}

		// Chuyển feedback của Reviewer sang cho vòng lặp tiếp theo
		fb = attemptFeedback{PreviousPlan: outcome.Plan, Review: verdict}
		priorFeedback = verdict.Feedback
	}

	return nil, ErrNutritionPlanGenerationFailed
}

func clonePlan(p *GeneratedMealPlan) *GeneratedMealPlan {
	if p == nil {
		return nil
	}
	encoded, err := json.Marshal(p)
	if err != nil {
		return p
	}
	var out GeneratedMealPlan
	if err := json.Unmarshal(encoded, &out); err != nil {
		return p
	}
	return &out
}

func reviewPasses(v *MealPlanReview) bool {
	if v == nil {
		return false
	}
	return v.Score >= approvalScore || v.Approved
}

func validateReview(v *MealPlanReview, prior []ReviewNote) error {
	if v == nil {
		return fmt.Errorf("%w: no verdict returned", errReviewUnusable)
	}
	if v.Score < 0 || v.Score > maxReviewScore {
		return fmt.Errorf("%w: score %d outside 0..%d", errReviewUnusable, v.Score, maxReviewScore)
	}
	if v.Confidence < 0 || v.Confidence > 1 {
		return fmt.Errorf("%w: confidence %.2f outside 0..1", errReviewUnusable, v.Confidence)
	}

	if err := checkFeedbackCompliance(v, prior); err != nil {
		return err
	}

	if reviewPasses(v) {
		return nil
	}

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

func checkFeedbackCompliance(v *MealPlanReview, prior []ReviewNote) error {
	if len(prior) == 0 {
		return nil
	}

	reported := make(map[string]bool, len(v.PreviousFeedback))
	for i, outcome := range v.PreviousFeedback {
		if outcome.Evidence == "" {
			return fmt.Errorf("%w: previous_feedback[%d] (%s) has no evidence",
				errReviewUnusable, i, outcome.Area)
		}
		reported[outcome.Area] = true
	}

	for _, note := range prior {
		if !reported[note.Area] {
			return fmt.Errorf("%w: previous round asked for %q and the verdict does not say whether it landed",
				errReviewUnusable, note.Area)
		}
	}

	if unapplied := v.unappliedAreas(); len(unapplied) > 0 && v.Approved {
		return fmt.Errorf("%w: approved with unapplied asks: %s",
			errReviewUnusable, strings.Join(unapplied, ", "))
	}

	return nil
}

func reviewIssueLines(v *MealPlanReview) []string {
	if v == nil {
		return nil
	}
	lines := make([]string, 0, len(v.Feedback))
	for _, f := range v.Feedback {
		lines = append(lines, fmt.Sprintf("review [%s]: %s (suggested fix: %s)", f.Area, f.Detail, f.Fix))
	}
	return lines
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
