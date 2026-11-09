package adk

import (
	"context"
	"errors"
	"testing"

	"github.com/viethung213/gym-companion/internal/nutrition/domain/aggregate"
	"github.com/viethung213/gym-companion/internal/nutrition/domain/vo"
)

func TestRunWithRetries_SuccessFirstAttempt(t *testing.T) {
	t.Parallel()

	foodChicken := aggregate.NewFoodItem("f1", "Ức gà", "PROTEIN", 165, 31, 0, 3.6, nil, "", "", false)
	foodCarb := aggregate.NewFoodItem("f2", "Khoai lang", "CARB", 90, 2, 21, 0.1, nil, "", "", false)
	repo := &mockFoodRepoForValidator{items: map[string]*aggregate.FoodItem{"f1": foodChicken, "f2": foodCarb}}

	validator := newPlanValidator(repo, vo.NewLockoutRegistry(nil))

	attemptFn := func(_ int, _ *attemptFeedback) (*GeneratedMealPlan, error) {
		return &GeneratedMealPlan{
			Options: []GeneratedMealOption{
				{
					ProteinFoodID:   "f1",
					ProteinFoodName: "Ức gà",
					CarbFoodID:      "f2",
					CarbFoodName:    "Khoai lang",
				},
				{
					ProteinFoodID:   "f2",
					ProteinFoodName: "Khoai lang",
					CarbFoodID:      "f1",
					CarbFoodName:    "Ức gà",
				},
			},
		}, nil
	}

	result, err := runWithRetries(context.Background(), validator, nil, nil, attemptFn, nil)
	if err != nil {
		t.Fatalf("unexpected error running retries: %v", err)
	}

	if result.Degraded {
		t.Fatalf("expected non-degraded result on first attempt success")
	}
}

func TestRunWithRetries_SalvageDegradedOnFinalAttempt(t *testing.T) {
	t.Parallel()

	foodChicken := aggregate.NewFoodItem("f1", "Ức gà", "PROTEIN", 165, 31, 0, 3.6, nil, "", "", false)
	repo := &mockFoodRepoForValidator{items: map[string]*aggregate.FoodItem{"f1": foodChicken}}

	validator := newPlanValidator(repo, vo.NewLockoutRegistry(nil))

	attemptFn := func(attempt int, _ *attemptFeedback) (*GeneratedMealPlan, error) {
		if attempt < 2 {
			return nil, errors.New("simulated AI failure")
		}
		// Attempt 2 returns valid options
		return &GeneratedMealPlan{
			Options: []GeneratedMealOption{
				{
					ProteinFoodID:   "f1",
					ProteinFoodName: "Ức gà",
				},
				{
					ProteinFoodID:   "f1",
					ProteinFoodName: "Ức gà",
				},
			},
		}, nil
	}

	result, err := runWithRetries(context.Background(), validator, nil, nil, attemptFn, nil)
	if err != nil {
		t.Fatalf("unexpected error running retries: %v", err)
	}

	if result.Plan == nil || len(result.Plan.Options) < 2 {
		t.Fatalf("expected salvaged plan with at least 2 options")
	}
}

func TestRunWithRetries_ReviewApproved(t *testing.T) {
	t.Parallel()

	foodChicken := aggregate.NewFoodItem("f1", "Ức gà", "PROTEIN", 165, 31, 0, 3.6, nil, "", "", false)
	repo := &mockFoodRepoForValidator{items: map[string]*aggregate.FoodItem{"f1": foodChicken}}

	validator := newPlanValidator(repo, vo.NewLockoutRegistry(nil))

	attemptFn := func(_ int, _ *attemptFeedback) (*GeneratedMealPlan, error) {
		return &GeneratedMealPlan{
			Options: []GeneratedMealOption{
				{ProteinFoodID: "f1", ProteinFoodName: "Ức gà", RecipeName: "Ức gà luộc"},
				{ProteinFoodID: "f1", ProteinFoodName: "Ức gà", RecipeName: "Ức gà xé phay"},
			},
		}, nil
	}

	reviewFn := func(_ int, _ *GeneratedMealPlan, _ ValidationReport, _ []ReviewNote) (*MealPlanReview, error) {
		return &MealPlanReview{
			Approved:   true,
			Score:      85,
			Confidence: 0.95,
			Notes:      []string{"Thực đơn ngon, đủ đạm"},
		}, nil
	}

	result, err := runWithRetries(context.Background(), validator, nil, nil, attemptFn, reviewFn)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result.Review == nil || !result.Review.Approved || result.Review.Score != 85 {
		t.Fatalf("expected approved review with score 85, got %v", result.Review)
	}
	if result.Degraded {
		t.Fatalf("expected non-degraded plan on review approval")
	}
}

func TestRunWithRetries_ReviewRejectionWithFeedbackAndCompliance(t *testing.T) {
	t.Parallel()

	foodChicken := aggregate.NewFoodItem("f1", "Ức gà", "PROTEIN", 165, 31, 0, 3.6, nil, "", "", false)
	repo := &mockFoodRepoForValidator{items: map[string]*aggregate.FoodItem{"f1": foodChicken}}

	validator := newPlanValidator(repo, vo.NewLockoutRegistry(nil))

	attempts := 0
	attemptFn := func(attempt int, fb *attemptFeedback) (*GeneratedMealPlan, error) {
		attempts++
		recipe := "Ức gà xào cay nồng"
		if fb != nil && fb.Review != nil && len(fb.Review.Feedback) > 0 {
			recipe = "Ức gà áp chảo thảo mộc thanh nhẹ"
		}
		return &GeneratedMealPlan{
			Options: []GeneratedMealOption{
				{ProteinFoodID: "f1", ProteinFoodName: "Ức gà", RecipeName: recipe},
				{ProteinFoodID: "f1", ProteinFoodName: "Ức gà", RecipeName: "Ức gà hấp"},
			},
		}, nil
	}

	reviewCalls := 0
	reviewFn := func(round int, plan *GeneratedMealPlan, _ ValidationReport, prior []ReviewNote) (*MealPlanReview, error) {
		reviewCalls++
		if round == 1 {
			return &MealPlanReview{
				Approved:   false,
				Score:      60,
				Confidence: 0.9,
				Feedback: []ReviewNote{
					{
						Area:   "culinary_harmony",
						Detail: "Gia vị quá cay nóng cho bữa sáng",
						Fix:    "Chuyển sang áp chảo thảo mộc thanh nhẹ",
					},
				},
			}, nil
		}

		// Round 2
		return &MealPlanReview{
			Approved:   true,
			Score:      80,
			Confidence: 0.95,
			PreviousFeedback: []PreviousFeedbackItem{
				{
					Area:     "culinary_harmony",
					Applied:  true,
					Evidence: "Option 1 recipe changed to: " + plan.Options[0].RecipeName,
				},
			},
			Notes: []string{"Đã sửa thành công"},
		}, nil
	}

	result, err := runWithRetries(context.Background(), validator, nil, nil, attemptFn, reviewFn)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if attempts != 2 {
		t.Fatalf("expected 2 attempts, got %d", attempts)
	}
	if reviewCalls != 2 {
		t.Fatalf("expected 2 review calls, got %d", reviewCalls)
	}
	if !result.Review.Approved {
		t.Fatalf("expected approved result on second round")
	}
}

func TestValidateReview_Checks(t *testing.T) {
	t.Parallel()

	// Nil review
	if err := validateReview(nil, nil); err == nil {
		t.Fatal("expected error for nil review")
	}

	// Score out of range
	invalidScore := &MealPlanReview{Score: 105, Confidence: 0.9}
	if err := validateReview(invalidScore, nil); err == nil {
		t.Fatal("expected error for score > 100")
	}

	// Confidence out of range
	invalidConf := &MealPlanReview{Score: 80, Confidence: 1.5}
	if err := validateReview(invalidConf, nil); err == nil {
		t.Fatal("expected error for confidence > 1")
	}

	// Rejected but no feedback
	rejectedNoFB := &MealPlanReview{Approved: false, Score: 60, Confidence: 0.8}
	if err := validateReview(rejectedNoFB, nil); err == nil {
		t.Fatal("expected error for rejection with no feedback")
	}

	// Rejected with feedback missing Fix
	feedbackNoFix := &MealPlanReview{
		Approved:   false,
		Score:      60,
		Confidence: 0.8,
		Feedback:   []ReviewNote{{Area: "culinary_harmony", Detail: "bad taste"}},
	}
	if err := validateReview(feedbackNoFix, nil); err == nil {
		t.Fatal("expected error for feedback missing Fix")
	}

	// Unapplied previous asks while claiming Approved
	prior := []ReviewNote{{Area: "cooking_feasibility", Detail: "too long", Fix: "shorten"}}
	approvedWithUnapplied := &MealPlanReview{
		Approved:   true,
		Score:      80,
		Confidence: 0.9,
		PreviousFeedback: []PreviousFeedbackItem{
			{Area: "cooking_feasibility", Applied: false, Evidence: "still 2h"},
		},
	}
	if err := validateReview(approvedWithUnapplied, prior); err == nil {
		t.Fatal("expected error when approving with unapplied feedback asks")
	}
}

func TestRunWithRetries_MaxTwoAttemptsEnforced(t *testing.T) {
	t.Parallel()

	foodChicken := aggregate.NewFoodItem("f1", "Ức gà", "PROTEIN", 165, 31, 0, 3.6, nil, "", "", false)
	repo := &mockFoodRepoForValidator{items: map[string]*aggregate.FoodItem{"f1": foodChicken}}
	validator := newPlanValidator(repo, vo.NewLockoutRegistry(nil))

	totalAttempts := 0
	attemptFn := func(attempt int, _ *attemptFeedback) (*GeneratedMealPlan, error) {
		totalAttempts = attempt
		return &GeneratedMealPlan{
			Options: []GeneratedMealOption{
				{ProteinFoodID: "f1", ProteinFoodName: "Ức gà", RecipeName: "Ức gà luộc"},
				{ProteinFoodID: "f1", ProteinFoodName: "Ức gà", RecipeName: "Ức gà xé"},
			},
		}, nil
	}

	reviewCalls := 0
	reviewFn := func(round int, _ *GeneratedMealPlan, _ ValidationReport, _ []ReviewNote) (*MealPlanReview, error) {
		reviewCalls = round
		// Always reject to see if it ever goes past attempt 2
		return &MealPlanReview{
			Approved:   false,
			Score:      50,
			Confidence: 0.9,
			Feedback: []ReviewNote{
				{OptionIndex: 1, Area: "culinary_harmony", Detail: "need fix", Fix: "change recipe"},
			},
		}, nil
	}

	result, err := runWithRetries(context.Background(), validator, nil, nil, attemptFn, reviewFn)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if totalAttempts > 2 {
		t.Fatalf("strictly enforced max 2 attempts, but executed %d", totalAttempts)
	}
	if reviewCalls > 2 {
		t.Fatalf("strictly enforced max 2 review calls, but executed %d", reviewCalls)
	}
	if !result.Degraded {
		t.Fatalf("expected degraded status after round 2 rejection without loop to round 3")
	}
}
