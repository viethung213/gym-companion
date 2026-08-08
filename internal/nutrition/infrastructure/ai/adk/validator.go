package adk

import (
	"context"
	"fmt"
	"strings"
	"time"

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

func (v *planValidator) validate(
	ctx context.Context,
	plan *GeneratedMealPlan,
	availableIngredients []FoodNutrientDTO,
	restrictions []string,
	isFinalAttempt bool,
) (*ValidationOutcome, error) {
	if plan == nil || len(plan.Options) == 0 {
		return &ValidationOutcome{
			Plan:   plan,
			Issues: []string{"generated plan is empty or nil"},
		}, nil
	}

	var issues []string
	if len(plan.Options) < 2 {
		issues = append(issues, fmt.Sprintf("generated plan must contain at least 2 meal options, got %d", len(plan.Options)))
	}
	validOptions := make([]GeneratedMealOption, 0, len(plan.Options))

	restrictionMap := make(map[string]bool)
	for _, r := range restrictions {
		restrictionMap[strings.ToUpper(strings.TrimSpace(r))] = true
	}

	for idx := range plan.Options {
		opt := &plan.Options[idx]
		optIssues := make([]string, 0)

		// 1. Verify & Auto-Heal Protein Food
		proteinInfo := v.resolveFoodItem(ctx, opt.ProteinFoodID, opt.ProteinFoodName, availableIngredients, plan.NewFoodCatalogItems)
		if proteinInfo == nil && (opt.ProteinFoodID != "" || opt.ProteinFoodName != "") {
			optIssues = append(optIssues, fmt.Sprintf("option %d: protein_food_id '%s' not found in active catalog", idx+1, opt.ProteinFoodID))
		} else if proteinInfo != nil {
			if proteinInfo.ID != "" {
				opt.ProteinFoodID = proteinInfo.ID
			}
			if proteinInfo.Name != "" {
				opt.ProteinFoodName = proteinInfo.Name
			}
			for _, tag := range proteinInfo.AllergenTags {
				if restrictionMap[strings.ToUpper(strings.TrimSpace(tag))] {
					optIssues = append(optIssues, fmt.Sprintf("option %d: protein_food '%s' contains restricted allergen '%s'", idx+1, proteinInfo.Name, tag))
				}
			}
		}

		// 2. Verify & Auto-Heal Carb Food
		carbInfo := v.resolveFoodItem(ctx, opt.CarbFoodID, opt.CarbFoodName, availableIngredients, plan.NewFoodCatalogItems)
		if carbInfo == nil && (opt.CarbFoodID != "" || opt.CarbFoodName != "") {
			optIssues = append(optIssues, fmt.Sprintf("option %d: carb_food_id '%s' not found in active catalog", idx+1, opt.CarbFoodID))
		} else if carbInfo != nil {
			if carbInfo.ID != "" {
				opt.CarbFoodID = carbInfo.ID
			}
			if carbInfo.Name != "" {
				opt.CarbFoodName = carbInfo.Name
			}
			for _, tag := range carbInfo.AllergenTags {
				if restrictionMap[strings.ToUpper(strings.TrimSpace(tag))] {
					optIssues = append(optIssues, fmt.Sprintf("option %d: carb_food '%s' contains restricted allergen '%s'", idx+1, carbInfo.Name, tag))
				}
			}
		}

		// 3. Verify & Auto-Heal Veggie Food
		veggieInfo := v.resolveFoodItem(ctx, opt.VeggieFoodID, opt.VeggieFoodName, availableIngredients, plan.NewFoodCatalogItems)
		if veggieInfo == nil && (opt.VeggieFoodID != "" || opt.VeggieFoodName != "") {
			optIssues = append(optIssues, fmt.Sprintf("option %d: veggie_food_id '%s' not found in active catalog", idx+1, opt.VeggieFoodID))
		} else if veggieInfo != nil {
			if veggieInfo.ID != "" {
				opt.VeggieFoodID = veggieInfo.ID
			}
			if veggieInfo.Name != "" {
				opt.VeggieFoodName = veggieInfo.Name
			}
			for _, tag := range veggieInfo.AllergenTags {
				if restrictionMap[strings.ToUpper(strings.TrimSpace(tag))] {
					optIssues = append(optIssues, fmt.Sprintf("option %d: veggie_food '%s' contains restricted allergen '%s'", idx+1, veggieInfo.Name, tag))
				}
			}
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

type validatedFoodInfo struct {
	ID           string
	Name         string
	AllergenTags []string
}

func (v *planValidator) resolveFoodItem(
	ctx context.Context,
	id, name string,
	availableIngredients []FoodNutrientDTO,
	newCatalogItems []NewFoodItemSpec,
) *validatedFoodInfo {
	// 1. Database lookup by ID
	if id != "" {
		item, err := v.foodRepo.FindByID(ctx, id)
		if err == nil && item != nil {
			return &validatedFoodInfo{
				ID:           item.ID(),
				Name:         item.Name(),
				AllergenTags: item.AllergenTags(),
			}
		}
	}

	// 2. Database lookup by Name
	if name != "" {
		item, err := v.foodRepo.FindByName(ctx, name)
		if err == nil && item != nil {
			return &validatedFoodInfo{
				ID:           item.ID(),
				Name:         item.Name(),
				AllergenTags: item.AllergenTags(),
			}
		}
	}

	// 3. Match in availableIngredients by ID
	if id != "" {
		for i := range availableIngredients {
			if availableIngredients[i].ID == id {
				return &validatedFoodInfo{
					ID:           availableIngredients[i].ID,
					Name:         availableIngredients[i].Name,
					AllergenTags: availableIngredients[i].AllergenTags,
				}
			}
		}
	}

	// 4. Match in availableIngredients by Name
	if name != "" {
		for i := range availableIngredients {
			if strings.EqualFold(availableIngredients[i].Name, name) {
				return &validatedFoodInfo{
					ID:           availableIngredients[i].ID,
					Name:         availableIngredients[i].Name,
					AllergenTags: availableIngredients[i].AllergenTags,
				}
			}
		}
	}

	// 5. Match in plan.NewFoodCatalogItems by Name
	if name != "" {
		for i := range newCatalogItems {
			if strings.EqualFold(newCatalogItems[i].Name, name) {
				foodID := id
				if foodID == "" {
					foodID = name
				}
				return &validatedFoodInfo{
					ID:           foodID,
					Name:         newCatalogItems[i].Name,
					AllergenTags: newCatalogItems[i].AllergenTags,
				}
			}
		}
	}

	return nil
}
