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

	attemptFn := func(attempt int, priorIssues []string) (*GeneratedMealPlan, error) {
		return &GeneratedMealPlan{
			Options: []GeneratedMealOption{
				{
					ProteinFoodID:   "f1",
					ProteinFoodName: "Ức gà",
					CarbFoodID:      "f2",
					CarbFoodName:    "Khoai lang",
				},
			},
		}, nil
	}

	result, err := runWithRetries(context.Background(), validator, nil, attemptFn)
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

	attemptFn := func(attempt int, priorIssues []string) (*GeneratedMealPlan, error) {
		if attempt < 3 {
			return nil, errors.New("simulated AI failure")
		}
		// Attempt 3 returns valid option
		return &GeneratedMealPlan{
			Options: []GeneratedMealOption{
				{
					ProteinFoodID:   "f1",
					ProteinFoodName: "Ức gà",
				},
			},
		}, nil
	}

	result, err := runWithRetries(context.Background(), validator, nil, attemptFn)
	if err != nil {
		t.Fatalf("unexpected error running retries salvage: %v", err)
	}

	if result.Plan == nil || len(result.Plan.Options) != 1 {
		t.Fatalf("expected salvaged plan with 1 option")
	}
}
