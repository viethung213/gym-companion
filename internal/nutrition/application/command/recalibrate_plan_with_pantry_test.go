package command_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/viethung213/gym-companion/internal/nutrition/application/command"
	"github.com/viethung213/gym-companion/internal/nutrition/domain/aggregate"
	"github.com/viethung213/gym-companion/internal/nutrition/domain/domainerror"
	"github.com/viethung213/gym-companion/internal/nutrition/domain/service"
	"github.com/viethung213/gym-companion/internal/nutrition/domain/vo"
)

func TestRecalibratePlanWithPantryHandler_Handle(t *testing.T) {
	t.Parallel()

	alloc, _ := vo.NewCalorieAllocation(2000, 150, 200, 66)
	optBreakfast := aggregate.NewMealOption("opt-bf", "Trứng ốp la", 400, 30, 30, 10, nil, nil, false)
	mealBreakfast := aggregate.NewDailyMeal("Breakfast", []aggregate.MealOption{optBreakfast})
	optLunch := aggregate.NewMealOption("opt-lunch", "Cơm gà luộc", 600, 45, 60, 15, nil, nil, false)
	mealLunch := aggregate.NewDailyMeal("Lunch", []aggregate.MealOption{optLunch})
	optDinner := aggregate.NewMealOption("opt-dinner", "Cá hồi áp chảo", 600, 45, 40, 20, nil, nil, false)
	mealDinner := aggregate.NewDailyMeal("Dinner", []aggregate.MealOption{optDinner})

	existingPlan := aggregate.NewNutritionPlan("plan-pantry-1", "user-pantry", time.Now(), alloc, []aggregate.DailyMeal{mealBreakfast, mealLunch, mealDinner})

	planRepo := &mockPlanRepo{plans: make(map[string]*aggregate.NutritionPlan)}
	_ = planRepo.Save(context.Background(), existingPlan)

	historyRepo := &mockHistoryRepo{}
	foodRepo := &mockFoodRepo{}
	recipeCacheRepo := &mockRecipeCacheRepo{}

	menuGen := service.NewMenuGenerator(recipeCacheRepo, foodRepo, &mockAIService{})

	handler := command.NewRecalibratePlanWithPantryHandler(planRepo, historyRepo, nil, menuGen)

	// 1. Kiểm tra khi thiếu MealType -> Báo lỗi ErrMealTypeRequired
	emptyMealTypeCmd := command.RecalibratePlanWithPantryCommand{
		UserID:               "user-pantry",
		PlanDate:             existingPlan.PlanDate(),
		MealType:             "",
		AvailableIngredients: []string{"Ức gà tươi"},
	}
	_, emptyErr := handler.Handle(context.Background(), &emptyMealTypeCmd)
	if !errors.Is(emptyErr, domainerror.ErrMealTypeRequired) {
		t.Fatalf("expected ErrMealTypeRequired on empty MealType, got %v", emptyErr)
	}

	// 2. Thành công: chỉ cập nhật bữa Lunch
	cmdObj := command.RecalibratePlanWithPantryCommand{
		UserID:               "user-pantry",
		PlanDate:             existingPlan.PlanDate(),
		MealType:             "Lunch",
		AvailableIngredients: []string{"Ức gà tươi", "Khoai lang luộc"},
	}

	recalibrated, err := handler.Handle(context.Background(), &cmdObj)
	if err != nil {
		t.Fatalf("unexpected error recalibrating plan with pantry: %v", err)
	}

	if recalibrated == nil {
		t.Fatalf("expected non-nil recalibrated plan")
	}

	// Bữa Breakfast và Dinner phải giữ nguyên
	meals := recalibrated.DailyMeals()
	if meals[0].MealType() != "Breakfast" || meals[0].Options()[0].OptionID() != "opt-bf" {
		t.Errorf("expected breakfast untouched, got %+v", meals[0])
	}
	if meals[2].MealType() != "Dinner" || meals[2].Options()[0].OptionID() != "opt-dinner" {
		t.Errorf("expected dinner untouched, got %+v", meals[2])
	}

	// 3. Kiểm tra khi người dùng gửi nguyên liệu nằm trong danh sách dị ứng
	allergyCmd := command.RecalibratePlanWithPantryCommand{
		UserID:               "user-pantry",
		PlanDate:             existingPlan.PlanDate(),
		MealType:             "Lunch",
		AvailableIngredients: []string{"Tôm sú", "Cơm trắng"},
		UserRestrictions:     []string{"TÔM"},
	}

	_, allergyErr := handler.Handle(context.Background(), &allergyCmd)
	if allergyErr == nil {
		t.Fatalf("expected error when sending allergenic ingredient, got nil")
	}
	if !errors.Is(allergyErr, domainerror.ErrAllergenConflict) {
		t.Fatalf("expected ErrAllergenConflict, got %v", allergyErr)
	}

	// 4. Invariant: Khi bữa đã được log -> từ chối và không gọi AI
	newLunchOptID := meals[1].Options()[0].OptionID()
	_, _ = recalibrated.MarkOptionLogged(newLunchOptID)
	_ = planRepo.Update(context.Background(), recalibrated)

	alreadyLoggedCmd := command.RecalibratePlanWithPantryCommand{
		UserID:               "user-pantry",
		PlanDate:             existingPlan.PlanDate(),
		MealType:             "Lunch",
		AvailableIngredients: []string{"Ức gà tươi"},
	}
	_, loggedErr := handler.Handle(context.Background(), &alreadyLoggedCmd)
	if !errors.Is(loggedErr, domainerror.ErrMealAlreadyLogged) {
		t.Fatalf("expected ErrMealAlreadyLogged on logged meal, got %v", loggedErr)
	}
}
