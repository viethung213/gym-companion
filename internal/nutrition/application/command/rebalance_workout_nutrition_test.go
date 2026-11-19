package command_test

import (
	"context"
	"testing"
	"time"

	"github.com/viethung213/gym-companion/internal/nutrition/application/command"
	"github.com/viethung213/gym-companion/internal/nutrition/domain/aggregate"
	"github.com/viethung213/gym-companion/internal/nutrition/domain/vo"
)

func TestRebalanceWorkoutNutritionHandler_Handle(t *testing.T) {
	t.Parallel()

	alloc, _ := vo.NewCalorieAllocation(2000, 150, 200, 66)

	optBreakfast := aggregate.NewMealOption("opt-bf", "Bánh mì ốp la", 400, 20, 50, 10, nil, nil, false)
	mealBreakfast := aggregate.NewDailyMeal("Breakfast", []aggregate.MealOption{optBreakfast})

	optLunch := aggregate.NewMealOption("opt-lunch", "Cơm gà xối mỡ", 600, 40, 70, 15, nil, nil, false)
	mealLunch := aggregate.NewDailyMeal("Lunch", []aggregate.MealOption{optLunch})

	optDinner := aggregate.NewMealOption("opt-dinner", "Bò lúc lắc", 600, 45, 50, 15, nil, nil, false)
	mealDinner := aggregate.NewDailyMeal("Dinner", []aggregate.MealOption{optDinner})

	plan := aggregate.NewNutritionPlan("plan-workout-1", "user-workout", time.Now(), alloc, []aggregate.DailyMeal{mealBreakfast, mealLunch, mealDinner})

	// Bữa sáng đã ăn
	_, _ = plan.MarkOptionLogged("opt-bf")

	planRepo := &mockPlanRepo{plans: make(map[string]*aggregate.NutritionPlan)}
	_ = planRepo.Save(context.Background(), plan)

	handler := command.NewRebalanceWorkoutNutritionHandler(planRepo, nil)

	cmd := &command.RebalanceWorkoutNutritionCommand{
		UserID:      "user-workout",
		PlanDate:    plan.PlanDate(),
		SessionID:   "session-abc",
		TotalVolume: 3000, // surplus: 150 + 3000*0.05 = 300 kcal
	}

	updatedPlan, err := handler.Handle(context.Background(), cmd)
	if err != nil {
		t.Fatalf("unexpected error rebalancing workout nutrition: %v", err)
	}

	// 2000 kcal + 300 kcal surplus = 2300 kcal
	if updatedPlan.CalorieAllocation().TargetCalories() != 2300 {
		t.Errorf("got target calories %f, want 2300", updatedPlan.CalorieAllocation().TargetCalories())
	}

	meals := updatedPlan.DailyMeals()
	// Bữa sáng đã ăn -> giữ nguyên 400 kcal
	if meals[0].Options()[0].Calories() != 400 {
		t.Errorf("expected breakfast untouched, got %f", meals[0].Options()[0].Calories())
	}

	// Bữa trưa và bữa tối chưa ăn -> được scale tăng calo
	scaleFactor := 2300.0 / 2000.0 // 1.15
	expectedLunchCal := 600.0 * scaleFactor
	if meals[1].Options()[0].Calories() != expectedLunchCal {
		t.Errorf("expected lunch scaled to %f, got %f", expectedLunchCal, meals[1].Options()[0].Calories())
	}
	expectedDinnerCal := 600.0 * scaleFactor
	if meals[2].Options()[0].Calories() != expectedDinnerCal {
		t.Errorf("expected dinner scaled to %f, got %f", expectedDinnerCal, meals[2].Options()[0].Calories())
	}
}

func TestCalculateSurplusCalories(t *testing.T) {
	t.Parallel()

	// 1. Zero volume -> default 250 kcal
	if cal := command.CalculateSurplusCalories(0); cal != 250 {
		t.Errorf("got %f, want 250", cal)
	}

	// 2. Normal volume: 2000kg -> 150 + 100 = 250 kcal
	if cal := command.CalculateSurplusCalories(2000); cal != 250 {
		t.Errorf("got %f, want 250", cal)
	}

	// 3. High volume: 8000kg -> 150 + 400 = 550 kcal
	if cal := command.CalculateSurplusCalories(8000); cal != 550 {
		t.Errorf("got %f, want 550", cal)
	}

	// 4. Max cap 600 kcal: 15000kg -> 150 + 750 = 900 -> capped at 600
	if cal := command.CalculateSurplusCalories(15000); cal != 600 {
		t.Errorf("got %f, want 600", cal)
	}
}
