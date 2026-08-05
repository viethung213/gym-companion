package aggregate_test

import (
	"testing"
	"time"

	"github.com/viethung213/gym-companion/internal/nutrition/domain/aggregate"
	"github.com/viethung213/gym-companion/internal/nutrition/domain/vo"
)

func TestNutritionPlan_Lifecycle(t *testing.T) {
	t.Parallel()

	alloc, err := vo.NewCalorieAllocation(2000, 150, 200, 66)
	if err != nil {
		t.Fatalf("unexpected allocation error: %v", err)
	}

	ing := aggregate.NewIngredientGram("Ức gà", 150, false)
	opt := aggregate.NewMealOption("opt-1", "Ức gà luộc", 400, 30, 0, 5, []aggregate.IngredientGram{ing}, []string{"Luộc 15p"}, false)
	meal := aggregate.NewDailyMeal("Breakfast", []aggregate.MealOption{opt})

	plan := aggregate.NewNutritionPlan("plan-1", "user-1", time.Now(), alloc, []aggregate.DailyMeal{meal})

	if got := plan.ID(); got != "plan-1" {
		t.Fatalf("got ID %q, want %q", got, "plan-1")
	}
	if got := plan.UserID(); got != "user-1" {
		t.Fatalf("got UserID %q, want %q", got, "user-1")
	}
	if got := len(plan.DailyMeals()); got != 1 {
		t.Fatalf("got daily meals len %d, want 1", got)
	}
}

func TestNutritionPlan_MarkOptionLogged(t *testing.T) {
	t.Parallel()

	alloc, _ := vo.NewCalorieAllocation(2000, 150, 200, 66)
	opt := aggregate.NewMealOption("opt-10", "Ức gà áp chảo", 500, 40, 30, 10, nil, nil, false)
	meal := aggregate.NewDailyMeal("Lunch", []aggregate.MealOption{opt})
	plan := aggregate.NewNutritionPlan("plan-2", "user-2", time.Now(), alloc, []aggregate.DailyMeal{meal})

	loggedOpt, err := plan.MarkOptionLogged("opt-10")
	if err != nil {
		t.Fatalf("unexpected error logging option: %v", err)
	}
	if !loggedOpt.IsLogged() {
		t.Fatalf("expected option to be logged")
	}

	// Logging second time should fail
	_, err = plan.MarkOptionLogged("opt-10")
	if err == nil {
		t.Fatalf("expected error logging already logged option, got nil")
	}

	// Logging non-existing option should fail
	_, err = plan.MarkOptionLogged("opt-999")
	if err == nil {
		t.Fatalf("expected error logging non-existing option, got nil")
	}
}

func TestNutritionPlan_UpdateRemainingUnconsumedMeals(t *testing.T) {
	t.Parallel()

	alloc1, _ := vo.NewCalorieAllocation(2000, 150, 200, 66)
	alloc2, _ := vo.NewCalorieAllocation(2200, 165, 220, 70)

	optLogged := aggregate.NewMealOption("opt-logged", "Món sáng", 400, 30, 40, 10, nil, nil, false)
	mealBreakfast := aggregate.NewDailyMeal("Breakfast", []aggregate.MealOption{optLogged})

	optOldLunch := aggregate.NewMealOption("opt-old-lunch", "Món trưa cũ", 600, 45, 60, 15, nil, nil, false)
	mealLunch := aggregate.NewDailyMeal("Lunch", []aggregate.MealOption{optOldLunch})

	plan := aggregate.NewNutritionPlan("plan-3", "user-3", time.Now(), alloc1, []aggregate.DailyMeal{mealBreakfast, mealLunch})
	_, _ = plan.MarkOptionLogged("opt-logged")

	optNewLunch := aggregate.NewMealOption("opt-new-lunch", "Món trưa mới", 700, 50, 70, 18, nil, nil, false)
	updatedLunch := aggregate.NewDailyMeal("Lunch", []aggregate.MealOption{optNewLunch})

	plan.UpdateRemainingUnconsumedMeals([]aggregate.DailyMeal{updatedLunch}, alloc2)

	if got := plan.CalorieAllocation().TargetCalories(); got != 2200 {
		t.Fatalf("got updated calories %f, want 2200", got)
	}

	meals := plan.DailyMeals()
	if meals[1].Options()[0].OptionID() != "opt-new-lunch" {
		t.Fatalf("got unconsumed meal option ID %q, want %q", meals[1].Options()[0].OptionID(), "opt-new-lunch")
	}
}
