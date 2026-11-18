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
	existingPlan := aggregate.NewNutritionPlan("plan-pantry-1", "user-pantry", time.Now(), alloc, nil)

	planRepo := &mockPlanRepo{plans: make(map[string]*aggregate.NutritionPlan)}
	_ = planRepo.Save(context.Background(), existingPlan)

	historyRepo := &mockHistoryRepo{}
	foodRepo := &mockFoodRepo{}
	recipeCacheRepo := &mockRecipeCacheRepo{}

	menuGen := service.NewMenuGenerator(recipeCacheRepo, foodRepo, &mockAIService{})

	handler := command.NewRecalibratePlanWithPantryHandler(planRepo, historyRepo, nil, menuGen)

	cmdObj := command.RecalibratePlanWithPantryCommand{
		UserID:               "user-pantry",
		PlanDate:             existingPlan.PlanDate(),
		AvailableIngredients: []string{"Ức gà tươi", "Khoai lang luộc"},
	}

	recalibrated, err := handler.Handle(context.Background(), &cmdObj)
	if err != nil {
		t.Fatalf("unexpected error recalibrating plan with pantry: %v", err)
	}

	if recalibrated == nil {
		t.Fatalf("expected non-nil recalibrated plan")
	}

	// Kiểm tra khi người dùng gửi nguyên liệu nằm trong danh sách dị ứng
	allergyCmd := command.RecalibratePlanWithPantryCommand{
		UserID:               "user-pantry",
		PlanDate:             existingPlan.PlanDate(),
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
}
