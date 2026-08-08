package adk

import (
	"context"
	"testing"
	"time"

	"github.com/viethung213/gym-companion/internal/nutrition/domain/aggregate"
	"github.com/viethung213/gym-companion/internal/nutrition/domain/vo"
)

type mockFoodRepoForValidator struct {
	items map[string]*aggregate.FoodItem
}

func (m *mockFoodRepoForValidator) FindByID(_ context.Context, id string) (*aggregate.FoodItem, error) {
	return m.items[id], nil
}
func (m *mockFoodRepoForValidator) FindByName(_ context.Context, name string) (*aggregate.FoodItem, error) {
	for _, item := range m.items {
		if item.Name() == name {
			return item, nil
		}
	}
	return nil, nil
}
func (m *mockFoodRepoForValidator) FindActiveCatalog(_ context.Context) ([]vo.FoodNutrient, error) {
	return nil, nil
}
func (m *mockFoodRepoForValidator) FindNutiFoodProducts(_ context.Context) ([]vo.FoodNutrient, error) {
	return nil, nil
}
func (m *mockFoodRepoForValidator) Save(_ context.Context, _ *aggregate.FoodItem) error   { return nil }
func (m *mockFoodRepoForValidator) Update(_ context.Context, _ *aggregate.FoodItem) error { return nil }

func TestPlanValidator_Validate(t *testing.T) {
	t.Parallel()

	foodChicken := aggregate.NewFoodItem("f-chicken", "Ức gà", "PROTEIN", 165, 31, 0, 3.6, []string{"Poultry"}, "CHICKEN", "", false)
	repo := &mockFoodRepoForValidator{items: map[string]*aggregate.FoodItem{"f-chicken": foodChicken}}

	lockout := vo.NewLockoutRegistry(nil).ApplyLockout(vo.LockoutTypeProtein, "Ức gà", 7*24*time.Hour, time.Now())
	validator := newPlanValidator(repo, lockout)

	plan := &GeneratedMealPlan{
		Options: []GeneratedMealOption{
			{
				ProteinFoodID:   "f-chicken",
				ProteinFoodName: "Ức gà",
				CarbFoodID:      "f-carb-invalid", // Not in DB
				CarbFoodName:    "Khoai lang",
			},
			{
				ProteinFoodID:   "f-fish",
				ProteinFoodName: "Cá hồi",
				CarbFoodID:      "f-rice",
				CarbFoodName:    "Cơm lứt",
			},
		},
	}

	outcome, err := validator.validate(context.Background(), plan, nil, []string{"Poultry"}, false)
	if err != nil {
		t.Fatalf("unexpected error validating: %v", err)
	}

	if len(outcome.Issues) == 0 {
		t.Fatalf("expected validation issues for allergen and missing carb_food_id")
	}
}

func TestPlanValidator_AutoHealing(t *testing.T) {
	t.Parallel()

	foodChicken := aggregate.NewFoodItem("real-uuid-chicken", "Ức gà tươi", "PROTEIN", 165, 31, 0, 3.6, nil, "CHICKEN", "", false)
	foodFish := aggregate.NewFoodItem("real-uuid-fish", "Cá hồi", "PROTEIN", 200, 20, 0, 12, nil, "FISH", "", false)
	repo := &mockFoodRepoForValidator{items: map[string]*aggregate.FoodItem{
		"real-uuid-chicken": foodChicken,
		"real-uuid-fish":    foodFish,
	}}
	validator := newPlanValidator(repo, vo.NewLockoutRegistry(nil))

	plan := &GeneratedMealPlan{
		Options: []GeneratedMealOption{
			{
				ProteinFoodID:   "PRO_01", // Dummy hallucinated ID
				ProteinFoodName: "Ức gà tươi",
			},
			{
				ProteinFoodID:   "real-uuid-fish",
				ProteinFoodName: "Cá hồi",
			},
		},
	}

	outcome, err := validator.validate(context.Background(), plan, nil, nil, false)
	if err != nil {
		t.Fatalf("unexpected error validating: %v", err)
	}

	if len(outcome.Issues) > 0 {
		t.Fatalf("expected 0 issues due to auto-healing, got: %v", outcome.Issues)
	}

	if plan.Options[0].ProteinFoodID != "real-uuid-chicken" {
		t.Fatalf("expected ProteinFoodID to be healed to 'real-uuid-chicken', got '%s'", plan.Options[0].ProteinFoodID)
	}
}

func TestPlanValidator_PantryAndNewFoodCatalogItems(t *testing.T) {
	t.Parallel()

	repo := &mockFoodRepoForValidator{items: make(map[string]*aggregate.FoodItem)}
	validator := newPlanValidator(repo, vo.NewLockoutRegistry(nil))

	availablePantry := []FoodNutrientDTO{
		{
			ID:              "pantry-bitter-melon-uuid",
			Name:            "Trái khổ qua",
			Category:        "VEGGIE",
			CaloriesPer100g: 17,
			ProteinPer100g:  1,
			CarbsPer100g:    3.7,
			FatPer100g:      0.2,
		},
	}

	plan := &GeneratedMealPlan{
		Options: []GeneratedMealOption{
			{
				ProteinFoodID:   "new-chicken-id",
				ProteinFoodName: "Ức gà sốt bơ",
				CarbFoodID:      "",
				CarbFoodName:    "Cơm gạo lứt",
				VeggieFoodID:    "pantry-bitter-melon-uuid",
				VeggieFoodName:  "Trái khổ qua",
			},
			{
				ProteinFoodID:   "new-chicken-id",
				ProteinFoodName: "Ức gà sốt bơ",
				CarbFoodID:      "",
				CarbFoodName:    "Cơm gạo lứt",
				VeggieFoodID:    "pantry-bitter-melon-uuid",
				VeggieFoodName:  "Trái khổ qua",
			},
		},
		NewFoodCatalogItems: []NewFoodItemSpec{
			{Name: "Ức gà sốt bơ", Category: "PROTEIN", CaloriesPer100g: 180, ProteinPer100g: 28},
			{Name: "Cơm gạo lứt", Category: "CARB", CaloriesPer100g: 110, CarbsPer100g: 23},
		},
	}

	outcome, err := validator.validate(context.Background(), plan, availablePantry, nil, false)
	if err != nil {
		t.Fatalf("unexpected error validating: %v", err)
	}

	if len(outcome.Issues) > 0 {
		t.Fatalf("expected 0 validation issues for pantry and new food items, got: %v", outcome.Issues)
	}

	if plan.Options[0].VeggieFoodID != "pantry-bitter-melon-uuid" {
		t.Fatalf("expected VeggieFoodID to match pantry UUID, got '%s'", plan.Options[0].VeggieFoodID)
	}
}
