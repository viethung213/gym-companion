package adk

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/viethung213/gym-companion/internal/nutrition/domain/aggregate"
	"github.com/viethung213/gym-companion/internal/nutrition/domain/repository"
	"github.com/viethung213/gym-companion/internal/nutrition/domain/vo"
)

type planValidator struct {
	foodRepo        repository.FoodItemRepository
	lockoutRegistry vo.LockoutRegistry
}

func newPlanValidator(foodRepo repository.FoodItemRepository, lockoutRegistry vo.LockoutRegistry) *planValidator {
	return &planValidator{
		foodRepo:        foodRepo,
		lockoutRegistry: lockoutRegistry,
	}
}

func (v *planValidator) validate(ctx context.Context, plan *GeneratedMealPlan, restrictions []string, isFinalAttempt bool) (*ValidationOutcome, error) {
	if plan == nil || len(plan.Options) == 0 {
		return &ValidationOutcome{
			Plan:   plan,
			Issues: []string{"generated plan is empty or nil"},
		}, nil
	}

	var issues []string
	validOptions := make([]GeneratedMealOption, 0, len(plan.Options))

	restrictionMap := make(map[string]bool)
	for _, r := range restrictions {
		restrictionMap[strings.ToUpper(strings.TrimSpace(r))] = true
	}

	for idx := range plan.Options {
		opt := &plan.Options[idx]
		optIssues := make([]string, 0)

		// 1. Verify & Auto-Heal Protein Food
		var proteinItem *aggregate.FoodItem
		if opt.ProteinFoodID != "" {
			item, err := v.foodRepo.FindByID(ctx, opt.ProteinFoodID)
			if err == nil && item != nil {
				proteinItem = item
			}
		}
		if proteinItem == nil && opt.ProteinFoodName != "" {
			item, err := v.foodRepo.FindByName(ctx, opt.ProteinFoodName)
			if err == nil && item != nil {
				proteinItem = item
				opt.ProteinFoodID = item.ID()
			}
		}
		if proteinItem == nil && (opt.ProteinFoodID != "" || opt.ProteinFoodName != "") {
			optIssues = append(optIssues, fmt.Sprintf("option %d: protein_food_id '%s' not found in active catalog", idx+1, opt.ProteinFoodID))
		} else if proteinItem != nil {
			for _, tag := range proteinItem.AllergenTags() {
				if restrictionMap[strings.ToUpper(strings.TrimSpace(tag))] {
					optIssues = append(optIssues, fmt.Sprintf("option %d: protein_food '%s' contains restricted allergen '%s'", idx+1, proteinItem.Name(), tag))
				}
			}
		}

		// 2. Verify & Auto-Heal Carb Food
		var carbItem *aggregate.FoodItem
		if opt.CarbFoodID != "" {
			item, err := v.foodRepo.FindByID(ctx, opt.CarbFoodID)
			if err == nil && item != nil {
				carbItem = item
			}
		}
		if carbItem == nil && opt.CarbFoodName != "" {
			item, err := v.foodRepo.FindByName(ctx, opt.CarbFoodName)
			if err == nil && item != nil {
				carbItem = item
				opt.CarbFoodID = item.ID()
			}
		}
		if carbItem == nil && (opt.CarbFoodID != "" || opt.CarbFoodName != "") {
			optIssues = append(optIssues, fmt.Sprintf("option %d: carb_food_id '%s' not found in active catalog", idx+1, opt.CarbFoodID))
		} else if carbItem != nil {
			for _, tag := range carbItem.AllergenTags() {
				if restrictionMap[strings.ToUpper(strings.TrimSpace(tag))] {
					optIssues = append(optIssues, fmt.Sprintf("option %d: carb_food '%s' contains restricted allergen '%s'", idx+1, carbItem.Name(), tag))
				}
			}
		}

		// 3. Verify & Auto-Heal Veggie Food
		var veggieItem *aggregate.FoodItem
		if opt.VeggieFoodID != "" {
			item, err := v.foodRepo.FindByID(ctx, opt.VeggieFoodID)
			if err == nil && item != nil {
				veggieItem = item
			}
		}
		if veggieItem == nil && opt.VeggieFoodName != "" {
			item, err := v.foodRepo.FindByName(ctx, opt.VeggieFoodName)
			if err == nil && item != nil {
				veggieItem = item
				opt.VeggieFoodID = item.ID()
			}
		}
		if veggieItem == nil && (opt.VeggieFoodID != "" || opt.VeggieFoodName != "") {
			optIssues = append(optIssues, fmt.Sprintf("option %d: veggie_food_id '%s' not found in active catalog", idx+1, opt.VeggieFoodID))
		}

		// 4. Verify Lockout Rules
		_, lockedCollisions := v.lockoutRegistry.CheckCollisions([]string{opt.ProteinFoodName, opt.CarbFoodName}, time.Now())
		if len(lockedCollisions) > 0 {
			optIssues = append(optIssues, fmt.Sprintf("option %d: ingredients %v are locked under 7/5/3 day repetition rules", idx+1, lockedCollisions))
		}

		if len(optIssues) > 0 {
			issues = append(issues, optIssues...)
		} else {
			validOptions = append(validOptions, *opt)
		}
	}

	if isFinalAttempt {
		return &ValidationOutcome{
			Plan:   &GeneratedMealPlan{Options: validOptions},
			Issues: issues,
		}, nil
	}

	return &ValidationOutcome{
		Plan:   plan,
		Issues: issues,
	}, nil
}
