package aggregate_test

import (
	"errors"
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

func TestNutritionPlan_UpdateMealOptions(t *testing.T) {
	t.Parallel()

	alloc, _ := vo.NewCalorieAllocation(2000, 150, 200, 66)

	optBreakfast := aggregate.NewMealOption("opt-bf", "Bữa sáng cũ", 400, 30, 40, 10, nil, nil, false)
	mealBreakfast := aggregate.NewDailyMeal("Breakfast", []aggregate.MealOption{optBreakfast})

	optOldLunch := aggregate.NewMealOption("opt-lunch-old", "Cơm tấm", 600, 45, 60, 15, nil, nil, false)
	mealLunch := aggregate.NewDailyMeal("Lunch", []aggregate.MealOption{optOldLunch})

	optDinner := aggregate.NewMealOption("opt-dinner", "Bò bít tết", 700, 50, 50, 20, nil, nil, false)
	mealDinner := aggregate.NewDailyMeal("Dinner", []aggregate.MealOption{optDinner})

	plan := aggregate.NewNutritionPlan("plan-single-meal", "user-sm", time.Now(), alloc, []aggregate.DailyMeal{mealBreakfast, mealLunch, mealDinner})

	// 1. Lỗi khi meal_type rỗng
	if err := plan.UpdateMealOptions("", nil); !errors.Is(err, aggregate.ErrMealTypeRequired) {
		t.Fatalf("expected ErrMealTypeRequired, got %v", err)
	}

	// 2. Lỗi khi meal_type không tồn tại
	if err := plan.UpdateMealOptions("MidnightSnack", nil); !errors.Is(err, aggregate.ErrMealTypeNotFound) {
		t.Fatalf("expected ErrMealTypeNotFound, got %v", err)
	}

	// 3. Cập nhật thành công cho bữa Lunch
	optNewLunch := aggregate.NewMealOption("opt-lunch-pantry", "Gà xào ớt chuông", 620, 50, 55, 12, nil, nil, false)
	if err := plan.UpdateMealOptions("LUNCH", []aggregate.MealOption{optNewLunch}); err != nil {
		t.Fatalf("unexpected error updating meal options: %v", err)
	}

	meals := plan.DailyMeals()
	// Bữa Breakfast phải giữ nguyên 100%
	if meals[0].MealType() != "Breakfast" || meals[0].Options()[0].OptionID() != "opt-bf" {
		t.Errorf("expected breakfast untouched, got %+v", meals[0])
	}
	// Bữa Lunch phải có options mới
	if meals[1].MealType() != "Lunch" || len(meals[1].Options()) != 1 || meals[1].Options()[0].OptionID() != "opt-lunch-pantry" {
		t.Errorf("expected lunch updated, got %+v", meals[1])
	}
	// Bữa Dinner phải giữ nguyên 100%
	if meals[2].MealType() != "Dinner" || meals[2].Options()[0].OptionID() != "opt-dinner" {
		t.Errorf("expected dinner untouched, got %+v", meals[2])
	}

	// 4. Invariant: Khi bữa đã log -> Từ chối cập nhật
	_, _ = plan.MarkOptionLogged("opt-lunch-pantry")
	optAnotherLunch := aggregate.NewMealOption("opt-lunch-another", "Phở bò", 650, 45, 70, 15, nil, nil, false)
	err := plan.UpdateMealOptions("Lunch", []aggregate.MealOption{optAnotherLunch})
	if !errors.Is(err, aggregate.ErrMealAlreadyLogged) {
		t.Fatalf("expected ErrMealAlreadyLogged when meal is logged, got %v", err)
	}
}

func TestNutritionPlan_IsMealLogged(t *testing.T) {
	t.Parallel()

	alloc, _ := vo.NewCalorieAllocation(2000, 150, 200, 66)
	opt := aggregate.NewMealOption("opt-1", "Phở", 500, 30, 60, 10, nil, nil, false)
	meal := aggregate.NewDailyMeal("Lunch", []aggregate.MealOption{opt})
	plan := aggregate.NewNutritionPlan("p-1", "u-1", time.Now(), alloc, []aggregate.DailyMeal{meal})

	isLogged, err := plan.IsMealLogged("Lunch")
	if err != nil || isLogged {
		t.Fatalf("expected not logged, got %v, err=%v", isLogged, err)
	}

	_, _ = plan.MarkOptionLogged("opt-1")
	isLogged, err = plan.IsMealLogged("LUNCH")
	if err != nil || !isLogged {
		t.Fatalf("expected logged, got %v, err=%v", isLogged, err)
	}

	_, err = plan.IsMealLogged("Dinner")
	if !errors.Is(err, aggregate.ErrMealTypeNotFound) {
		t.Fatalf("expected ErrMealTypeNotFound, got %v", err)
	}
}

func TestNutritionPlan_AdjustWorkoutSurplus(t *testing.T) {
	t.Parallel()

	alloc, _ := vo.NewCalorieAllocation(2000, 150, 200, 66)

	optBf := aggregate.NewMealOption("opt-bf", "Bánh cuốn", 400, 20, 50, 10, nil, nil, false)
	mealBf := aggregate.NewDailyMeal("Breakfast", []aggregate.MealOption{optBf})

	optDinner := aggregate.NewMealOption("opt-dinner", "Bò áp chảo", 600, 45, 50, 15, []aggregate.IngredientGram{
		aggregate.NewIngredientGram("Thịt bò", 150, false),
	}, nil, false)
	mealDinner := aggregate.NewDailyMeal("Dinner", []aggregate.MealOption{optDinner})

	plan := aggregate.NewNutritionPlan("p-surplus", "u-surplus", time.Now(), alloc, []aggregate.DailyMeal{mealBf, mealDinner})

	// Giả lập bữa sáng đã ăn
	_, _ = plan.MarkOptionLogged("opt-bf")

	// Điều chỉnh bù 400 kcal sau buổi tập
	if err := plan.AdjustWorkoutSurplus(400); err != nil {
		t.Fatalf("unexpected error adjusting surplus: %v", err)
	}

	// CalorieAllocation phải được cập nhật tăng từ 2000 lên 2400
	if plan.CalorieAllocation().TargetCalories() != 2400 {
		t.Errorf("got target calories %f, want 2400", plan.CalorieAllocation().TargetCalories())
	}

	meals := plan.DailyMeals()
	// Bữa sáng đã ăn -> giữ nguyên 400 kcal
	if meals[0].Options()[0].Calories() != 400 {
		t.Errorf("expected logged breakfast calories unchanged, got %f", meals[0].Options()[0].Calories())
	}

	// Bữa tối chưa ăn -> được scale theo hệ số 2400/2000 = 1.2 -> 600 * 1.2 = 720 kcal
	if meals[1].Options()[0].Calories() != 720 {
		t.Errorf("expected unconsumed dinner calories scaled to 720, got %f", meals[1].Options()[0].Calories())
	}
	// Định lượng thịt bò 150g -> 180g
	if meals[1].Options()[0].Ingredients()[0].Grams() != 180 {
		t.Errorf("expected beef grams scaled to 180, got %f", meals[1].Options()[0].Ingredients()[0].Grams())
	}
}
