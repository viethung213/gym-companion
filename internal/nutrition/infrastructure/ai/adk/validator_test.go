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
func (m *mockFoodRepoForValidator) FindByName(_ context.Context, _ string) (*aggregate.FoodItem, error) {
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
		},
	}

	outcome, err := validator.validate(context.Background(), plan, []string{"Poultry"}, false)
	if err != nil {
		t.Fatalf("unexpected error validating: %v", err)
	}

	if len(outcome.Issues) == 0 {
		t.Fatalf("expected validation issues for allergen and missing carb_food_id")
	}
}
